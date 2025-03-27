package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// VitaConfig is the configuration file format used to configure the Vita
// backend application.
type VitaConfig struct {
	TemplateDirectory string `mapstructure:"template_directory"`
	MongoUri          string `mapstructure:"mongo_uri"`
	BindAddress       string `mapstructure:"bind_address"`
}

// GetTemplates tries to look up all defined templates in the given
// TemplateDirectory from the VitaConfig. It does this by iterating
// over the subdirectories therein, and reading the 'template.toml'
// in those dirs.
//
// The function returns a map where the key is the subdirectory name,
// and the TemplateConfiguration, which is the deserialized toml file.
func (cfg *VitaConfig) GetTemplates() (map[string]TemplateConfiguration, error) {

	absolutePath, err := filepath.Abs(cfg.TemplateDirectory)
	if err != nil {
		return nil, err
	}

	log.Printf("Reading templates from template directory %s", absolutePath)

	templateDirectoryFiles, err := os.ReadDir(absolutePath)
	if err != nil {
		return nil, err
	}

	templateMap := make(map[string]TemplateConfiguration)

	// Oh man I love Go's simplicity, but this err handling is bonkers sometimes,
	// I swear. It's easy to reason about though, I'll give you that. Anyway,
	// I could have used fs.WalkDir, but I only need to recurse max 2 directories
	// deep.
	for _, subFile := range templateDirectoryFiles {
		if subFile.IsDir() {
			templateDir := filepath.Join(cfg.TemplateDirectory, subFile.Name())
			subdir, err := os.ReadDir(templateDir)
			if err != nil {
				log.Printf("Unable to read subdir '%s' : %s", templateDir, err)
				continue
			}

			for _, file := range subdir {
				if file.Type().IsRegular() && file.Name() == "template.toml" {
					templateConfigFile := filepath.Join(cfg.TemplateDirectory, subFile.Name(), "template.toml")

					abs, _ := filepath.Abs(templateConfigFile)
					log.Printf("Parsing template from subdir '%s'", abs)
					contents, err := os.ReadFile(templateConfigFile)
					if err != nil {
						log.Printf("Could not read template.toml file from '%s': %s", templateConfigFile, err)
						continue
					}
					var tmplConfig TemplateConfiguration
					tmplConfig.Id = subFile.Name()
					err = toml.Unmarshal(contents, &tmplConfig)
					if err != nil {
						log.Printf("Could not unmarshal template.toml file from '%s': %s", templateConfigFile, err)
						continue
					}

					templateMap[templateDir] = tmplConfig
				}
			}
		}
	}

	log.Printf("Found %d templates", len(templateMap))

	return templateMap, nil
}
