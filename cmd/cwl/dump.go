package main

import (
  "fmt"
  "encoding/json"
  "github.com/buchanae/cwl"
  "github.com/spf13/cobra"
)

type dumpOpts struct {
  resolveSchemaDefs bool
}

func init() {
  opts := dumpOpts{}

  cmd := &cobra.Command{
    Use: "dump <doc.cwl>",
    Args: cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
      return dump(opts, args[0])
    },
  }
  root.AddCommand(cmd)

  f := cmd.Flags()
  f.BoolVar(&opts.resolveSchemaDefs, "resolve-schema-defs", opts.resolveSchemaDefs, "")
}

func dump(opts dumpOpts, path string) error {
  doc, err := cwl.Load(path)
  if err != nil {
    return err
  }

  // TODO also resolve http/file references for schema types?
  if opts.resolveSchemaDefs {
    if tool, ok := doc.(*cwl.Tool); ok {
      err := tool.ResolveSchemaDefs()
      if err != nil {
        return err
      }
    }
  }

  b, err := json.MarshalIndent(doc, "", "  ")
  if err != nil {
    return err
  }

  fmt.Println(string(b))
  return nil
}
