package main

import (
	"archive/zip"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type functionConfig struct {
	Name     string `yaml:"name"`
	FileName string `yaml:"fileName"`
	Path     string `yaml:"-"`
}

func main() {
	currentDir, err := os.Getwd()
	if err != nil {
		logrus.WithError(err).Fatal("error while reading current directory")
	}

	files, err := findFunctionConfigs(currentDir)
	if err != nil {
		logrus.WithError(err).Fatal("error while reading function files directory")
	}

	for _, file := range files {
		func() {
			config, err := parseFunctionConfig(file)
			if err != nil {
				logrus.WithError(err).Fatalf("error while reading function config at %s", file)
			}

			if err := config.build(); err != nil {
				logrus.WithError(err).Fatalf("error while compiling for config at %s", file)
			}
			defer config.mustDeleteBuildFile()

			if err := config.zipBuild(); err != nil {
				logrus.WithError(err).Fatalf("error while building function config at %s", file)
			}
			defer config.mustDeleteZipFile()

			if err := config.updateLambda(); err != nil {
				logrus.WithError(err).Fatalf("error while updating Lambda-Function for config at %s", file)
			}
		}()
	}
}

// getBuildOutputPath returns the path where the built function file should be written to.
func (conf *functionConfig) getBuildOutputPath() string {
	return fmt.Sprintf("%s/%s", conf.Path, conf.Name)
}

// getZipOutputPath returns the path where the zipped built should be written to.
func (conf *functionConfig) getZipOutputPath() string {
	return fmt.Sprintf("%s/%s.zip", conf.Path, conf.Name)
}

// mustDeleteBuildFile deletes the built file for this functionConfig.
func (conf *functionConfig) mustDeleteBuildFile() {
	if err := os.Remove(conf.getBuildOutputPath()); err != nil {
		logrus.WithError(err).Fatalf("error while deleting build at %s", conf.getBuildOutputPath())
	}
}

func (conf *functionConfig) getFullFilePath() string {
	return fmt.Sprintf("%s/%s", conf.Path, conf.FileName)
}

// mustDeleteBuildFile deletes the built file for this functionConfig.
func (conf *functionConfig) mustDeleteZipFile() {
	if err := os.Remove(conf.getZipOutputPath()); err != nil {
		logrus.WithError(err).Fatalf("error while deleting build at %s", conf.getBuildOutputPath())
	}
}

// build runs the go build command for the referenced source file.
// Returns the path of the output file.
func (conf *functionConfig) build() error {
	if err := exec.Command("go", "build", "-o", conf.getBuildOutputPath(), conf.getFullFilePath()).Run(); err != nil {
		return err
	}
	return nil
}

// zipBuild puts the built for this functionConfig into a zip file.
func (conf *functionConfig) zipBuild() error {
	zipFile, err := os.Create(conf.getZipOutputPath())
	if err != nil {
		return err
	}

	writer := zip.NewWriter(zipFile)
	defer writer.Close()

	fileToZip, err := os.Open(conf.getBuildOutputPath())
	if err != nil {
		return err
	}
	defer fileToZip.Close()

	fileStats, err := fileToZip.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(fileStats)
	if err != nil {
		return err
	}

	header.Name = fileStats.Name()
	header.Method = zip.Deflate

	fileWriter, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(fileWriter, fileToZip)
	if err != nil {
		return err
	}

	return nil
}

// updateLambda takes the built and zipped go file and updates the corresponding Lambda function.
// This functions also checks if the handler name is still correct.
func (conf *functionConfig) updateLambda() error {
	data, err := ioutil.ReadFile(conf.getZipOutputPath())
	if err != nil {
		return err
	}

	sess := session.Must(session.NewSession())

	lambdaSess := lambda.New(sess)

	lambdaInfo, err := lambdaSess.UpdateFunctionCode(&lambda.UpdateFunctionCodeInput{
		FunctionName: &conf.Name,
		ZipFile:      data,
	})

	if err != nil {
		return err
	}
	logrus.Infof("updated lambda function %s", *lambdaInfo.FunctionName)

	// Check if the handler name is still correct of if it must be updated
	if strings.Compare(*lambdaInfo.Handler, conf.Name) != 0 {
		_, err := lambdaSess.UpdateFunctionConfiguration(&lambda.UpdateFunctionConfigurationInput{
			Handler: &conf.Name,
		})
		if err != nil {
			return err
		}
		logrus.Infof("updated handler name for lambda %s to prevent issues", *lambdaInfo.FunctionName)
	}

	return nil
}

// findFunctionConfigs searches recursively starting a root directory.
// returns a slice of found function configs.
func findFunctionConfigs(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.Compare(info.Name(), ".function.yaml") == 0 {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err

	}
	return files, nil
}

// parseFunctionConfig parses a .function.yaml file at the given path
func parseFunctionConfig(path string) (*functionConfig, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var function functionConfig
	if err := yaml.Unmarshal(data, &function); err != nil {
		return nil, err
	}

	function.Path = strings.Replace(path, "/.function.yaml", "", 1)

	return &function, nil
}
