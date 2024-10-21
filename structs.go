package main

type vars struct {
	File string
	List *[]string
}

type processContext struct {
	SourceArray *[]string
	TargetArray *[]string
	Source      string
	Target      string
	Debug       bool
}

type fileVars struct {
	Action       Action
	Source       string
	Target       string
	ListToCopy   []string
	ListToDelete []string
	Debug        bool
}

type Args struct {
	Source   string
	Target   string
	Interval int64
	Debug    bool
}

type Action int

const (
	DELETE Action = iota
	COPY
	RESOLVE
)
