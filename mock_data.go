package mocka

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// MockData is used to import the mock data from a yaml file.
// THe query and response are stored as values in the file, as it
// was getting a bit unwieldy to store longer queries as the key as
// was done in the earlier versions.
type MockData struct {
	Query    string   `yaml:"query"`
	Response Response `yaml:"response"`
}

type MockDataFile struct {
	Data []MockData `yaml:"data"`
}

// // MockUserData is used to import the mock data from a yaml file.
// type MockUserData struct {
// 	User     string     `yaml:"user"`
// 	MockData []MockData `yaml:"mocks"`
// }

type MockUserDataFile map[string]MockDataFile

func (m MockDataFile) AsResponseMap() ResponseMap {
	responseMap := make(ResponseMap)
	for _, data := range m.Data {
		responseMap[data.Query] = data.Response
	}
	return responseMap
}

func (m MockUserDataFile) AsUserResponseMap() UserResponseMap {
	userResponseMap := make(UserResponseMap)
	for user, data := range m {
		userResponseMap[user] = data.AsResponseMap()
	}
	return userResponseMap
}

func ResponseMapFromFile(path string) (ResponseMap, error) {
	var mockDataFile MockDataFile
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read file %s: %w", path, err)
	}
	err = yaml.Unmarshal(contents, &mockDataFile)
	if err != nil {
		return nil, fmt.Errorf("could not parse file %s: %w", path, err)
	}
	return mockDataFile.AsResponseMap(), nil
}

func UserResponseMapFromFile(path string) (UserResponseMap, error) {
	var mockDataFile MockUserDataFile
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read file %s: %w", path, err)
	}
	err = yaml.Unmarshal(contents, &mockDataFile)
	if err != nil {
		return nil, fmt.Errorf("could not parse file %s: %w", path, err)
	}
	return mockDataFile.AsUserResponseMap(), nil
}
