package model

import (
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type App struct {
	Name         string    `json:"name"`
	Version      *string   `json:"version"`
	Pid          *int      `json:"pid"`
	ProcessTitle *string   `json:"process_title"`
	Argv         []string  `json:"argv"`
	Language     Language  `json:"language"`
	Runtime      Runtime   `json:"runtime"`
	Framework    Framework `json:"framework"`
	Agent        Agent     `json:"agent"`
	GitRef       *string   `json:"git_ref"`
}

type Language struct {
	Name    *string `json:"name"`
	Version *string `json:"version"`
}
type Runtime struct {
	Name    *string `json:"name"`
	Version *string `json:"version"`
}
type Framework struct {
	Name    *string `json:"name"`
	Version *string `json:"version"`
}
type Agent struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type TransformApp func(a *App) common.MapStr

func (a *App) MinimalTransform() common.MapStr {
	enhancer := utility.NewMapStrEnhancer()
	app := common.MapStr{
		"name": a.Name,
		"agent": common.MapStr{
			"name":    a.Agent.Name,
			"version": a.Agent.Version,
		},
	}
	enhancer.Add(app, "git_ref", a.GitRef)
	return app
}

func (a *App) Transform() common.MapStr {
	enhancer := utility.NewMapStrEnhancer()
	app := a.MinimalTransform()
	enhancer.Add(app, "version", a.Version)
	enhancer.Add(app, "pid", a.Pid)
	enhancer.Add(app, "process_title", a.ProcessTitle)
	enhancer.Add(app, "argv", a.Argv)

	lang := common.MapStr{}
	enhancer.Add(lang, "name", a.Language.Name)
	enhancer.Add(lang, "version", a.Language.Version)
	enhancer.Add(app, "language", lang)

	runtime := common.MapStr{}
	enhancer.Add(runtime, "name", a.Runtime.Name)
	enhancer.Add(runtime, "version", a.Runtime.Version)
	enhancer.Add(app, "runtime", runtime)

	framework := common.MapStr{}
	enhancer.Add(framework, "name", a.Framework.Name)
	enhancer.Add(framework, "version", a.Framework.Version)
	enhancer.Add(app, "framework", framework)

	return app
}
