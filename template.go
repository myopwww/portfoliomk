package ortfomk

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/chai2010/gettext-go/po"
	"github.com/davecgh/go-spew/spew"
	"github.com/jaytaylor/html2text"
)

// GetTemplateFuncMap returns the funcmap used to hydrate files
func GetTemplateFuncMap(language string, data *GlobalData) template.FuncMap {
	return template.FuncMap{
		// translate translates the given string into `language`
		"translate": data.TranslateFunction(language),
		// "with" filters a []WorkOneLang
		"withTag":       withTag,
		"withTech":      withTech,
		"withWIPStatus": withWIPStatus,
		"excluding":     excluding,
		// Nice, cosy aliases for filters
		"withCreatedYear": withCreatedYear,
		"tagged":          withTag,
		"madeWith":        withTech,
		"createdIn":       withCreatedYear,
		"finished": func(ws []WorkOneLang) []WorkOneLang {
			return withWIPStatus(false, ws)
		},
		"unfinished": func(ws []WorkOneLang) []WorkOneLang {
			return withWIPStatus(true, ws)
		},
		// reduces a []WorkOneLang down to a single WorkOneLang
		"latest": latest,
		// functions acting on paths
		"asset": asset,
		"media": media,
		// lookups for tags & technologies
		"lookupTag":  func(name string) Tag { return lookupTag(data.Tags, name) },
		"lookupTech": func(name string) Technology { return lookupTech(data.Technologies, name) },
		// debugging
		"log": log,
		// various
		"makeWS":       makeWorkSlice,
		"appendWS":     appendWorkSlice,
		"yearsOfWorks": yearsOfWorks,
		"tnindent":     tnindent,
	}
}

// TemplateData holds all of the data used to hydrate web pages
type TemplateData struct {
	Age               uint8
	KnownTags         []Tag
	KnownTechnologies []Technology
	Works             []WorkOneLang
	MusicTag          Tag
	// Template data for _-prefixed .pug files: relevant struct instance of what's being hydrated
	CurrentTag  Tag
	CurrentTech Technology
	CurrentWork WorkOneLang
}

// GetAge returns my age
func GetAge() uint8 {
	// TODO Do it dynamically
	return 17
}

// TranslateFunction returns a function that calls gettext to translate a string to the given language
func (t *Translations) TranslateFunction(language string) func(string) string {
	if language == "fr" {
		return func(text string) string {
			if translated := t.GetTranslation(text); translated != "" {
				return translated
			}
			t.missingMessages = append(t.missingMessages, po.Message{
				MsgId: text,
			})
			return text
		}
	}
	return func(text string) string { return text }
}

func (w WorkOneLang) ColorsCSS() string {
	var cssDeclaration string
	for key, value := range w.ColorsMap() {
		cssDeclaration += fmt.Sprintf("--%s:%s;", key, value)
	}
	return cssDeclaration
}

// getColorsMap returns a mapping of "primary", "secondary", etc to the color values,
// with an added "#" prefix if needed
func (w WorkOneLang) ColorsMap() map[string]string {
	colorsMap := make(map[string]string, 3)
	if w.Metadata.Colors.Primary != "" {
		colorsMap["primary"] = AddOctothorpeIfNeeded(w.Metadata.Colors.Primary)
	}
	if w.Metadata.Colors.Secondary != "" {
		colorsMap["secondary"] = AddOctothorpeIfNeeded(w.Metadata.Colors.Secondary)
	}
	if w.Metadata.Colors.Tertiary != "" {
		colorsMap["tertiary"] = AddOctothorpeIfNeeded(w.Metadata.Colors.Tertiary)
	}
	return colorsMap
}

// getSummary summarizes the given work's first description paragraph
func (w WorkOneLang) Summary() string {
	if len(w.Paragraphs) == 0 {
		return ""
	}
	plainText, err := html2text.FromString(w.Paragraphs[0].Content, html2text.Options{OmitLinks: true})
	if err != nil {
		panic(err)
	}
	return SummarizeString(plainText, 150)
}

func (w WorkOneLang) ThumbnailSource(resolution uint16) string {
	if len(w.Media) == 0 {
		return ""
	}
	if resolution > 0 {
		if thumbSource, ok := w.Metadata.Thumbnails[w.Media[0].Path][resolution]; ok {
			// FIXME: media/ shouldn't be hardcoded
			// Could be implemented by reading .portfolioortfodb.yaml
			// Therefore there should be a config file common to ortfo{db,mk}, just put .ortfo.yaml in the portfolio's root.
			thumbSource = strings.TrimPrefix(thumbSource, "media/")
			return media(thumbSource)
		}
	}
	return media(w.Media[0].Path)
}

func (c LayedOutCell) ThumbnailSource(resolution uint16) string {
	if resolution > 0 {
		if thumbSource, ok := c.Metadata.Thumbnails[c.Source][resolution]; ok {
			// FIXME: media/ shouldn't be hardcoded
			thumbSource = strings.TrimPrefix(thumbSource, "media/")
			return media(thumbSource)
		}
	}
	return media(c.Path)
}

func yearsOfWorks(ws []WorkOneLang) []int {
	years := make([]int, 0)
	for _, work := range ws {
		var isDuplicate bool
		for _, year := range years {
			if work.Created().Year() == year {
				isDuplicate = true
				continue
			}
		}
		if !isDuplicate {
			years = append(years, work.Created().Year())
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))
	return years
}

// withTag returns an array of works that have tag in their tags
func withTag(tag Tag, ws []WorkOneLang) []WorkOneLang {
	filtered := make([]WorkOneLang, 0)
	for _, work := range ws {
		for _, tagName := range work.Metadata.Tags {
			if tag.ReferredToBy(tagName) {
				filtered = append(filtered, work)
			}
		}
	}
	if len(filtered) == 0 {
		wsIDs := make([]string, len(ws))
		for _, work := range ws {
			wsIDs = append(wsIDs, work.ID)
		}
		// printfln("WARNING: No works from %v have the %s tag", wsIDs, tag.URLName())
	}
	return filtered
}

// withTag returns an array of works that have tech in their "made with" technologies list
func withTech(tech Technology, ws []WorkOneLang) []WorkOneLang {
	filtered := make([]WorkOneLang, 0)
	for _, work := range ws {
		for _, techName := range work.Metadata.MadeWith {
			if tech.ReferredToBy(techName) {
				filtered = append(filtered, work)
			}
		}
	}
	return filtered
}

func withWIPStatus(wipStatus bool, ws []WorkOneLang) []WorkOneLang {
	filtered := make([]WorkOneLang, 0)
	for _, work := range ws {
		if work.IsWIP() == wipStatus {
			filtered = append(filtered, work)
		}
	}
	return filtered
}

func withCreatedYear(createdYear int, ws []WorkOneLang) []WorkOneLang {
	filtered := make([]WorkOneLang, 0)
	for _, work := range ws {
		if work.Created().Year() == createdYear {
			filtered = append(filtered, work)
		}
	}
	return filtered
}

// excluding returns the given works excluding those given in excludelist, comparing IDs.
func excluding(excludelist []WorkOneLang, ws []WorkOneLang) []WorkOneLang {
	filtered := make([]WorkOneLang, len(ws))
	for _, work := range ws {
		excluded := false
		for _, excludedWork := range excludelist {
			if work.ID == excludedWork.ID {
				excluded = true
			}
		}
		if !excluded {
			filtered = append(filtered, work)
		}
	}
	return filtered
}

func latest(ws []WorkOneLang) WorkOneLang {
	if len(ws) == 0 {
		panic("cannot get the latest element of an empty array")
	}
	latest := ws[0]
	for _, work := range ws {
		if work.Created().Year() == 65535 {
			continue
		}
		if work.Created().After(latest.Created()) {
			latest = work
		}
	}
	return latest
}

func log(o interface{}) string {
	spew.Dump(o)
	if !BuildingForProduction() {
		return fmt.Sprintf("logged %v to stdout", o)
	}
	return ""
}

// asset returns the full URL for a given asset (ie a website's static asset like an icon)
func asset(assetPath string) string {
	assetPath = strings.ReplaceAll(assetPath, "#", "sharp")
	var urlScheme string
	if !BuildingForProduction() {
		urlScheme = "file://" + os.Getenv("LOCAL_PROJECTS_DIR") + "portfolio/assets/%s"
	} else {
		urlScheme = "https://assets.ewen.works/%s"
	}
	return fmt.Sprintf(urlScheme, assetPath)
}

// media returns the full URL for a given media (ie a work's media URL)
func media(mediaPath string) string {
	mediaPath = strings.ReplaceAll(mediaPath, "#", "sharp")
	var urlScheme string
	if !BuildingForProduction() {
		// FIXME: /media/ shouldn't be hardcoded.
		urlScheme = "file://" + os.Getenv("LOCAL_PROJECTS_DIR") + "/portfolio/media/%s"
	} else {
		urlScheme = "https://media.ewen.works/%s"
	}
	urlScheme = strings.ReplaceAll(urlScheme, "//", "/")
	urlScheme = strings.Replace(urlScheme, "https:/", "https://", 1)
	urlScheme = strings.Replace(urlScheme, "file:/", "file://", 1)
	return fmt.Sprintf(urlScheme, mediaPath)
}

// lookupTag returns the tag referred to by name
func lookupTag(tags []Tag, name string) Tag {
	for _, tag := range tags {
		if tag.ReferredToBy(name) {
			return tag
		}
	}
	panic("cannot find tag with display name " + name)
}

// lookupTech returns the tech referred to by name
func lookupTech(techs []Technology, name string) Technology {
	for _, tech := range techs {
		if tech.ReferredToBy(name) {
			return tech
		}
	}
	panic("cannot find tech with display name " + name)
}

func makeWorkSlice() []WorkOneLang {
	s := make([]WorkOneLang, 0)
	return s
}

func appendWorkSlice(toAppend WorkOneLang, ws []WorkOneLang) []WorkOneLang {
	new := append(ws, toAppend)
	return new
}

func tnindent(tabs int, s string) string {
	pad := strings.Repeat("\t", tabs)
	return "\n" + pad + strings.ReplaceAll(s, "\n", "\n"+pad)
}
