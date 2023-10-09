package plugin

import (
	"fmt"
	"github.com/samber/lo"
	"strings"
)

type WoxImageType = string

const (
	WoxImageTypeAbsolutePath = "absolute"
	WoxImageTypeRelativePath = "relative"
	WoxImageTypeBase64       = "base64"
	WoxImageTypeSvg          = "svg"
	WoxImageTypeRemote       = "remote"
)

type WoxImage struct {
	ImageType WoxImageType //see WoxImageType
	ImageData string
}

func NewWoxImage(imageType string, imageData string) WoxImage {
	return WoxImage{
		ImageType: imageType,
		ImageData: imageData,
	}
}

func (w *WoxImage) String() string {
	return fmt.Sprintf("%s:%s", w.ImageType, w.ImageData)
}

func ParseWoxImage(imageString string) (WoxImage, error) {
	var terms = strings.SplitN(imageString, ":", 1)
	if len(terms) != 2 {
		return WoxImage{}, fmt.Errorf("invalid image string: %s", imageString)
	}

	var w WoxImage
	w.ImageType = terms[0]
	w.ImageData = terms[1]
	return w, nil
}

// Query from Wox. See "Doc/Query.md" for details.
type Query struct {
	// Raw query, this includes trigger keyword if it has.
	// We didn't recommend use this property directly. You should always use Search property.
	RawQuery string

	// Trigger keyword of a query. It can be empty if user is using global trigger keyword.
	// Empty trigger keyword means this query will be a global query.
	TriggerKeyword string

	// Command part of a query.
	// Empty command means this query doesn't have a command.
	Command string

	// Search part of a query.
	// Empty search means this query doesn't have a search part.
	Search string
}

// Query result return from plugin
type QueryResult struct {
	// Result id, should be unique. It's optional, if you don't set it, Wox will assign a random id for you
	Id                     string
	Title                  string
	SubTitle               string
	Icon                   WoxImage
	Score                  int
	PreventHideAfterAction bool // If true, Wox will not hide after user select this result
	Action                 func()
}

type QueryResultUI struct {
	Id              string
	Title           string
	SubTitle        string
	Icon            string
	Score           int
	AssociatedQuery string
}

func NewQuery(query string) Query {
	var terms = strings.Split(query, " ")
	if len(terms) == 0 {
		return Query{
			RawQuery: query,
		}
	}

	var rawQuery = query
	var triggerKeyword, command, search string
	var possibleTriggerKeyword = terms[0]
	var mustContainSpace = strings.Contains(query, " ")

	pluginInstance, found := lo.Find(GetPluginManager().GetPluginInstances(), func(instance *Instance) bool {
		return lo.Contains(instance.GetTriggerKeywords(), possibleTriggerKeyword)
	})
	if found && mustContainSpace {
		// non global trigger keyword
		triggerKeyword = possibleTriggerKeyword

		if len(terms) == 1 {
			// no command and search
			command = ""
			search = ""
		} else {
			var possibleCommand = terms[1]
			if lo.ContainsBy(pluginInstance.Metadata.Commands, func(item MetadataCommand) bool {
				return item.Command == possibleCommand
			}) {
				// command and search
				command = possibleCommand
				search = strings.Join(terms[2:], " ")
			} else {
				// no command, only search
				command = ""
				search = strings.Join(terms[1:], " ")
			}
		}
	} else {
		// non trigger keyword
		triggerKeyword = ""
		command = ""
		search = rawQuery
	}

	return Query{
		RawQuery:       query,
		TriggerKeyword: triggerKeyword,
		Command:        command,
		Search:         search,
	}
}
