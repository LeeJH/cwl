package cwl

type Script struct {
	CWLVersion string `json:"cwlVersion,omitempty"`
	ID         string `json:"id,omitempty"`
	Label      string `json:"label,omitempty"`
	Doc        string `json:"doc,omitempty"`

	Hints        []Requirement `json:"hints,omitempty"`
	Requirements []Requirement `json:"requirements,omitempty"`

	Inputs  []ScriptInput  `json:"inputs,omitempty"`
	Outputs []ScriptOutput `json:"outputs,omitempty"`

	CSteps  []CStep   `json:"csteps,omitempty"`

	Stdin  Expression `json:"stdin,omitempty"`
	Stderr Expression `json:"stderr,omitempty"`
	Stdout Expression `json:"stdout,omitempty"`

	SuccessCodes       []int `json:"successCodes,omitempty"`
	TemporaryFailCodes []int `json:",omitempty"`
	PermanentFailCodes []int `json:",omitempty"`
}

type ScriptInput struct {
	ID         string `json:"id,omitempty"`
	Label      string `json:"label,omitempty"`
	Doc        string `json:"doc,omitempty"`
	Streamable bool   `json:"streamable,omitempty"`
	Default    Value  `json:"default,omitempty"`

	Type []InputType `json:"type,omitempty"`

	SecondaryFiles []Expression `json:"secondaryFiles,omitempty"`
	Format         []Expression `json:"format,omitempty"`

	InputBinding *CommandLineBinding `json:"inputBinding,omitempty"`
}

type ScriptOutput struct {
	ID         string `json:"id,omitempty"`
	Label      string `json:"label,omitempty"`
	Doc        string `json:"doc,omitempty"`
	Streamable bool   `json:"streamable,omitempty"`

	Type []OutputType `json:"type,omitempty"`

	SecondaryFiles []Expression `json:"secondaryFiles,omitempty"`
	Format         []Expression `json:"format,omitempty"`

	OutputBinding *CommandOutputBinding `json:"outputBinding,omitempty"`
}

type CStep struct {
	BaseCommand []string              `json:"baseCommand,omitempty"`
	Arguments   []*CommandLineBinding `json:"arguments,omitempty"`
}

