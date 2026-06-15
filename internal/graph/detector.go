package graph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type FrameworkRule struct {
	Framework string
	Detect    func(dir string) bool
}

type FrameworkDetector struct {
	rules []FrameworkRule
}

func NewFrameworkDetector(rules []FrameworkRule) *FrameworkDetector {
	return &FrameworkDetector{rules: rules}
}

func (d *FrameworkDetector) Detect(workspaceDir string) []string {
	var frameworks []string
	for _, rule := range d.rules {
		if rule.Detect(workspaceDir) {
			frameworks = append(frameworks, rule.Framework)
		}
	}
	return frameworks
}

var DefaultRules = []FrameworkRule{
	{Framework: "echo", Detect: detectEcho},
	{Framework: "gin", Detect: detectGin},
	{Framework: "express", Detect: detectExpress},
	{Framework: "nestjs", Detect: detectNestJS},
	{Framework: "nuxt", Detect: detectNuxt},
	{Framework: "rails", Detect: detectRails},
	{Framework: "go", Detect: detectGoModExists},
}

func detectGoModDep(dir string, depPath string) bool {
	if checkGoModDep(dir, depPath) {
		return true
	}
	return searchSubdirsForGoModDep(dir, depPath)
}

func checkGoModDep(dir string, depPath string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), depPath)
}

func searchSubdirsForGoModDep(dir string, depPath string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "node_modules" || name == ".git" || name == ".opencode" || name == "vendor" {
			continue
		}
		if checkGoModDep(filepath.Join(dir, name), depPath) {
			return true
		}
	}
	return false
}

func detectEcho(dir string) bool {
	return detectGoModDep(dir, "github.com/labstack/echo")
}

func detectGin(dir string) bool {
	return detectGoModDep(dir, "github.com/gin-gonic/gin")
}

func detectGoModExists(dir string) bool {
	if checkGoModExists(dir) {
		return true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "node_modules" || name == ".git" || name == ".opencode" || name == "vendor" {
			continue
		}
		if checkGoModExists(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

func checkGoModExists(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "module ")
}

func detectExpress(dir string) bool {
	if checkPackageJSONForDep(dir, "express") {
		return true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "node_modules" || name == ".git" || name == ".opencode" || name == "vendor" {
			continue
		}
		if checkPackageJSONForDep(filepath.Join(dir, name), "express") {
			return true
		}
	}
	return false
}

func detectNuxt(dir string) bool {
	if checkPackageJSONForDep(dir, "nuxt") {
		return true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "node_modules" || name == ".git" || name == ".opencode" || name == "vendor" {
			continue
		}
		if checkPackageJSONForDep(filepath.Join(dir, name), "nuxt") {
			return true
		}
	}
	return false
}

func detectRails(dir string) bool {
	if checkGemfileForRails(dir) {
		return true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "node_modules" || name == ".git" || name == ".opencode" || name == "vendor" {
			continue
		}
		if checkGemfileForRails(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

func checkGemfileForRails(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "Gemfile"))
	if err != nil {
		return false
	}
	content := string(data)
	return strings.Contains(content, "'rails'") || strings.Contains(content, "\"rails\"")
}

func detectNestJS(dir string) bool {
	if checkPackageJSONForNestJSDep(dir) {
		return true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "node_modules" || name == ".git" || name == ".opencode" || name == "vendor" {
			continue
		}
		if checkPackageJSONForNestJSDep(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

func checkPackageJSONForNestJSDep(dir string) bool {
	return checkPackageJSONForDep(dir, "@nestjs/common") || checkPackageJSONForDep(dir, "@nestjs/core")
}

func checkPackageJSONForDep(dir string, dep string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return false
	}
	var pkg struct {
		Dependencies   map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}
	if _, ok := pkg.Dependencies[dep]; ok {
		return true
	}
	if _, ok := pkg.DevDependencies[dep]; ok {
		return true
	}
	return false
}
