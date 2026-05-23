package summarize

import _ "embed"

//go:embed prompts/summarize.txt
var summarizePrompt string

//go:embed prompts/combine.txt
var combinePrompt string
