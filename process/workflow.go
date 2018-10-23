package process

import (
  "fmt"
  "strings"
  "github.com/spf13/cast"
  "github.com/google/uuid"
	"path/filepath"
  "cwl"
  "cwl/expr"
)

type scope struct {
  prefix string
  links map[string][]string
}
func (n scope) child(prefix string) scope {
  return scope{prefix: n.prefix + "/" + prefix, links: n.links}
}
func (n scope) link(name string, val string) {
  key := n.key(name)
  n.links[key] = append(n.links[key], val)
}
func (n scope) key(name string) string {
  return n.prefix + "/" + name
}


// DebugWorkflow is a temporary placeholder for workflow processing code.
func DebugWorkflow(wf *cwl.Workflow, vals cwl.Values) {

  root := scope{prefix: "", links: map[string][]string{}}

  inputs := root.child("inputs")
  for k, _ := range vals {
    root.link(k, inputs.key(k))
  }

  exports := linkWorkflow(wf, root)
  outputs := root.child("outputs")

  for _, out := range wf.Outputs {
    outputs.link(out.ID, exports.key(out.ID))
  }

  walk(root.links, []string{"/outputs/count_output"})

}

func walk(links map[string][]string, keys []string) {
  for _, key := range keys {
    fmt.Println(key)
    walk(links, links[key])
  }
}

func linkWorkflow(wf *cwl.Workflow, parent scope) scope {

  internal := parent.child("workflow")
  for _, in := range wf.Inputs {
    internal.link(in.ID, parent.key(in.ID))
  }

  for _, step := range wf.Steps {
    stepScope := internal.child("step/" + step.ID)

    for _, in := range step.In {
      for _, src := range in.Source {
        stepScope.link(in.ID, internal.key(src))
      }
    }

    stepExports := linkDoc(step.Run, stepScope)

    for _, out := range step.Out {
      id := step.ID + "/" + out.ID
      internal.link(id, stepExports.key(out.ID))
    }
  }

  exports := internal.child("exports")
  for _, out := range wf.Outputs {
    for _, src := range out.OutputSource {
      exports.link(out.ID, internal.key(src))
    }
  }

  return exports
}

func linkTool(in []cwl.CommandInput, out []cwl.CommandOutput, parent scope) scope {
  internal := parent.child("tool")
  for _, in := range in {
    internal.link(in.ID, parent.key(in.ID))
    internal.link("toolexec", internal.key(in.ID))
  }
  exports := internal.child("exports")
  for _, out := range out {
    exports.link(out.ID, internal.key("toolexec"))
  }
  return exports
}

func linkDoc(doc cwl.Document, parent scope) scope {
  switch z := doc.(type) {
  case *cwl.Workflow:
    return linkWorkflow(z, parent)
  case *cwl.Tool:
    return linkTool(z.Inputs, z.Outputs, parent)
  case *cwl.ExpressionTool:
    return linkTool(z.Inputs, z.Outputs, parent)
  }
  return scope{}
}

/*
TODO goals

- validate that links are correct, not missing any links, etc
- have (un)marshal-able workflow state
- validate value bindings, mid workflow
- resolve inputs to step in nested workflow, mid workflow
- encode links between steps directly, without intermediate layers


implementation thoughts:
- major element of name translation over many layers
- end result is link between two Process objects and/or
  Step objects.
- possibly want global Start/End steps, or maybe only End;
  End.Done() is true when the workflow is done. need to
  also have link between workflow outputs and last steps
- want to query value of value by name at any layer?
  e.g. query for workflow.step0.count_output mid workflow
*/

type WFProcess struct {
	wf             *cwl.Workflow
	inputs         cwl.Values
	runtime        Runtime
	fs             Filesystem
	bindings       []*Binding

	inputfiles		map[string][]cwl.File
	outputfiles 	[]cwl.File
	// End
	expressionLibs []string

}

func WFNewProcess(wf *cwl.Workflow, values cwl.Values, rt Runtime, fs Filesystem) (*WFProcess, error) {

  /*
  //TODO: Validate workflow
	err := cwl.ValidateTool(tool)
	if err != nil {
		return nil, err
	}
  */
	// TODO expose input bindings as an exported type of data
	//      could be useful to know separately from all the other processing.
	process := &WFProcess{
		wf:    wf,
		inputs:  values,
		runtime: rt,
		fs:      fs,
	}

	//TODO: Set default input values of workflow.
	//setDefaults(values, tool.Inputs)

	// Bind inputs to values.
	//
	// Since every part of a tool depends on "inputs" being available to expressions,
	// nothing can be done on a Process without a valid inputs binding,
	// which is why we bind in the Process constructor.
	
	for _, in := range wf.Inputs {
		val := values[in.ID]
		k := sortKey{getPos(in.InputBinding)}
		b, err := process.bindInput(in.ID, in.Type, in.InputBinding, in.SecondaryFiles, val, k)
		if err != nil {
			return nil, errf("binding input %q: %s", in.ID, err)
		}
		if b == nil {
			return nil, errf("no binding found for input: %s", in.ID)
		}

		process.bindings = append(process.bindings, b...)
	}




	files := map[string][]cwl.File{}
	for _, in := range process.InputBindings() {
	  if f, ok := in.Value.(cwl.File); ok {
		files[in.name] = flattenFiles(f)
	  }
	}
	process.inputfiles = files

	return process, nil
}

func (process *WFProcess) InputBindings() []*Binding {
	// TODO copying slice, but still using pointers. deep copy?
	bindings := make([]*Binding, len(process.bindings))
	copy(bindings, process.bindings)
	return bindings
}

// bindInput binds an input descriptor to a concrete value.
//
// bindInput is called recursively for types which have subtypes,
// such as array, record, etc.
//
// `name` is the field or parameter name.
// `types` is the list of types allowed by this input.
// `clb` is the cwl.CommandLineBinding describing how to bind this input.
// `val` is the input value for this input key.
// `key` is the sort key of the parent of this binding.
func (process *WFProcess) bindInput(
	name string,
	types []cwl.InputType,
	clb *cwl.CommandLineBinding,
	secondaryFiles []cwl.Expression,
	val interface{},
	key sortKey,
) ([]*Binding, error) {

	// If no value was found, check if the type is allowed to be null.
	// If so, return a binding, otherwise fail.
	if val == nil {
		for _, t := range types {
			if z, ok := t.(cwl.Null); ok {
				return []*Binding{
					{clb, z, nil, key, nil, name},
				}, nil
			}
		}
		return nil, errf("missing value")
	}

Loop:

	// An input descriptor describes multiple allowed types.
	// Loop over the types, looking for the best match for the given input value.
	for _, t := range types {
		switch z := t.(type) {

		case cwl.InputArray:
			vals, ok := val.([]cwl.Value)
			if !ok {
				// input value is not an array.
				continue Loop
			}

			// The input array is allowed to be empty,
			// so this must be a non-nil slice.
			out := []*Binding{}

			for i, val := range vals {
				subkey := append(key, sortKey{getPos(z.InputBinding), i}...)
				b, err := process.bindInput("", z.Items, z.InputBinding, nil, val, subkey)
				if err != nil {
					return nil, err
				}
				if b == nil {
					// array item values did not bind to the array descriptor.
					continue Loop
				}
				out = append(out, b...)
			}

			nested := make([]*Binding, len(out))
			copy(nested, out)
			b := &Binding{clb, z, val, key, nested, name}
			// TODO revisit whether creating a nested tree (instead of flat) is always better/ok
			return []*Binding{b}, nil

		case cwl.InputRecord:
			vals, ok := val.(map[string]cwl.Value)
			if !ok {
				// input value is not a record.
				continue Loop
			}

			var out []*Binding

			for i, field := range z.Fields {
				val, ok := vals[field.Name]
				// TODO lower case?
				if !ok {
					continue Loop
				}

				subkey := append(key, sortKey{getPos(field.InputBinding), i}...)
				b, err := process.bindInput(field.Name, field.Type, field.InputBinding, nil, val, subkey)
				if err != nil {
					return nil, err
				}
				if b == nil {
					continue Loop
				}
				out = append(out, b...)
			}

			if out != nil {
				nested := make([]*Binding, len(out))
				copy(nested, out)
				b := &Binding{clb, z, val, key, nested, name}
				out = append(out, b)
				return out, nil
			}

		case cwl.Any:
			return []*Binding{
				{clb, z, val, key, nil, name},
			}, nil

		case cwl.Boolean:
			v, err := cast.ToBoolE(val)
			if err != nil {
				continue Loop
			}
			return []*Binding{
				{clb, z, v, key, nil, name},
			}, nil

		case cwl.Int:
			v, err := cast.ToInt32E(val)
			if err != nil {
				continue Loop
			}
			return []*Binding{
				{clb, z, v, key, nil, name},
			}, nil

		case cwl.Long:
			v, err := cast.ToInt64E(val)
			if err != nil {
				continue Loop
			}
			return []*Binding{
				{clb, z, v, key, nil, name},
			}, nil

		case cwl.Float:
			v, err := cast.ToFloat32E(val)
			if err != nil {
				continue Loop
			}
			return []*Binding{
				{clb, z, v, key, nil, name},
			}, nil

		case cwl.Double:
			v, err := cast.ToFloat64E(val)
			if err != nil {
				continue Loop
			}
			return []*Binding{
				{clb, z, v, key, nil, name},
			}, nil

		case cwl.String:
			v, err := cast.ToStringE(val)
			if err != nil {
				continue Loop
			}

			return []*Binding{
				{clb, z, v, key, nil, name},
			}, nil

		case cwl.FileType:
			v, ok := val.(cwl.File)
			if !ok {
				continue Loop
			}

			f, err := process.resolveFile(v, clb.GetLoadContents())
			if err != nil {
				return nil, err
			}
			// TODO figure out a good way to do this.
			//f.Path = "/inputs/" + f.Path			
			for _, expr := range secondaryFiles {
				process.resolveSecondaryFiles(f, expr)
			}

			return []*Binding{
				{clb, z, f, key, nil, name},
			}, nil

		case cwl.DirectoryType:
			v, ok := val.(cwl.Directory)
			if !ok {
				continue Loop
			}
			// TODO resolve directory
			return []*Binding{
				{clb, z, v, key, nil, name},
			}, nil

		}
	}

	return nil, errf("missing value")
}

// resolveFile uses the filesystem to fill in all fields in the File,
// such as dirname, checksum, size, etc. If f.Contents is given, the
// file will be created via fs.Create(). if `loadContents` is true,
// the file contents will be loaded via fs.Contents().
func (process *WFProcess) resolveFile(f cwl.File, loadContents bool) (cwl.File, error) {
	// TODO revisit pointer to File
	var x cwl.File

	// http://www.commonwl.org/v1.0/CommandLineTool.html#File
	// "As a special case, if the path field is provided but the location field is not,
	// an implementation may assign the value of the path field to location,
	// and remove the path field."
	if f.Location == "" && f.Path != "" && f.Contents == "" {
		f.Location = f.Path
		f.Path = ""
	}

	if f.Location == "" && f.Contents == "" {
		return x, errf("location and contents are empty")
	}

	// If both location and contents are set, one will get overwritten.
	// Can't know which one the caller intended, so fail instead.
	if f.Location != "" && f.Contents != "" {
		return x, errf("location and contents are both non-empty")
	}

	var err error

	if f.Contents != "" {
		// Determine the file path of the literal.
		// Use the path, or the basename, or generate a random name.
		path := f.Path
		if path == "" {
			path = f.Basename
		}
		if path == "" {
			id, err := uuid.NewRandom()
			if err != nil {
				return x, errf("generating a random name for a file literal: %s", err)
			}
			path = id.String()
		}

		x, err = process.fs.Create(path, f.Contents)
		if err != nil {
			return x, errf("creating file from inline content: %s", err)
		}

	} else {
		// Only the local filesystem case implemented.
		x, err = process.fs.Info(f.Location)
		if err != nil {
			return x, errf("getting file info for %q: %s", f.Location, err)
		}

		if loadContents {
			f.Contents, err = process.fs.Contents(f.Location)
			if err != nil {
				return x, errf("loading file contents: %s", err)
			}
		}
	}

	// TODO clean this up. "x" was needed before a package reorg.
	//      possibly can be removed now.
	f.Location = x.Location
	// TODO figure out how to stage files.
	//      namespace inputs so they don't conflict.
	//      remember, the args building depends on this path, so it must happen
	//      in the Process code.
	//f.Path = filepath.Join("/inputs", filepath.Base(x.Path))
	//f.Path = filepath.Base(x.Path)
	f.Path = x.Path
	f.Checksum = x.Checksum
	f.Size = x.Size

	// cwl spec:
	// "If basename is provided, it is not required to match the value from location"
	if f.Basename == "" {
		f.Basename = filepath.Base(f.Path)
	}
	f.Nameroot, f.Nameext = splitname(f.Basename)
	f.Dirname = filepath.Dir(f.Path)

	return f, nil
}

func (process *WFProcess) resolveSecondaryFiles(file cwl.File, x cwl.Expression) error {

	// cwl spec:
	// "If the value is an expression, the value of self in the expression
	// must be the primary input or output File object to which this binding applies.
	// The basename, nameroot and nameext fields must be present in self.
	// For CommandLineTool outputs the path field must also be present.
	// The expression must return a filename string relative to the path
	// to the primary File, a File or Directory object with either path
	// or location and basename fields set, or an array consisting of strings
	// or File or Directory objects. It is legal to reference an unchanged File
	// or Directory object taken from input as a secondaryFile.
	// TODO
	if expr.IsExpression(x) {
		process.eval(x, file)
	}

	// cwl spec:
	// "If a value in secondaryFiles is a string that is not an expression,
	// it specifies that the following pattern should be applied to the location
	// of the primary file to yield a filename relative to the primary File:"

	// "If string begins with one or more caret ^ characters, for each caret,
	// remove the last file extension from the location (the last period . and all
	// following characters).
	pattern := string(x)
	// TODO location or path? cwl spec says "path" but I'm suspicious.
	location := file.Location

	for strings.HasPrefix(pattern, "^") {
		pattern = strings.TrimPrefix(pattern, "^")
		location = strings.TrimSuffix(location, filepath.Ext(location))
	}

	// "Append the remainder of the string to the end of the file location."
	sec := cwl.File{
		Location: location + pattern,
	}

	// TODO does LoadContents apply to secondary files? not in the spec
	f, err := process.resolveFile(sec, false)
	if err != nil {
		return err
	}

	file.SecondaryFiles = append(file.SecondaryFiles, f)
	return nil
}

func (process *WFProcess) eval(x cwl.Expression, self interface{}) (interface{}, error) {

	inputsData := map[string]interface{}{}
	for _, b := range process.bindings {
		v, err := toJSONMap(b.Value)
		if err != nil {
			return nil, wrap(err, `mashaling "%s" for JS eval`, b.name)
		}
		if v == nil {
			v = expr.Null
		}
		inputsData[b.name] = v
	}

	selfData, err := toJSONMap(self)
	if err != nil {
		return nil, wrap(err, `marshaling "self" for JS eval`)
	}

	r := process.runtime
	return expr.Eval(x, process.expressionLibs, map[string]interface{}{
		"inputs": inputsData,
		"self":   selfData,
		"runtime": map[string]interface{}{
			"outdir":     r.Outdir,
			"tmpdir":     r.Tmpdir,
			"cores":      r.Cores,
			"ram":        r.RAM,
			"outdirSize": r.OutdirSize,
			"tmpdirSize": r.TmpdirSize,
		},
	})
}