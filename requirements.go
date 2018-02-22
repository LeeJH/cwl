package cwl

type Hint interface {
	hint()
}

type Requirement interface {
	requirement()
}

type DockerRequirement struct {
	Hint
	Requirement

	Pull            string `cwl:"dockerPull"`
	Load            string `cwl:"dockerLoad"`
	File            string `cwl:"dockerFile"`
	Import          string `cwl:"dockerImport"`
	ImageID         string `cwl:"dockerImageID"`
	OutputDirectory string `cwl:"dockerOutputDirectory"`
}

type ResourceRequirement struct {
	Hint
	Requirement

	CoresMin Expression
	// TODO this is incorrectly denoted in the spec as int | string | expression
	CoresMax  Expression
	RAMMin    Expression
	RAMMax    Expression
	TmpDirMin Expression
	TmpDirMax Expression
	OutDirMin Expression
	OutDirMax Expression
}

type EnvDef struct{}

type EnvVarRequirement struct {
	Class  string
	EnvDef EnvDef
}

type EnvironmentDef struct {
	EnvName  string
	EnvValue Expression
}

type ShellCommandRequirement struct {
}

type InlineJavascriptRequirement struct {
	ExpressionLib []string
}

func (InlineJavascriptRequirement) hint()        {}
func (InlineJavascriptRequirement) requirement() {}

type SchemaDefRequirement struct {
	Types []InputSchema
}

type Packages struct {
}

type SoftwareRequirement struct {
	Packages Packages
}

type SoftwarePackage struct {
	Package string
	Version []string
	Specs   []string
}

type InitialWorkDirListing struct {
}

type InitialWorkDirRequirement struct {
	Listing InitialWorkDirListing
}

type Dirent struct {
	Entry     Expression
	Entryname Expression
	Writable  bool
}

type SubworkflowFeatureRequirement struct {
}

type ScatterFeatureRequirement struct {
}

type MultipleInputFeatureRequirement struct {
}

type StepInputExpressionRequirement struct {
}
