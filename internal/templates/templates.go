package templates

import "strings"

// Names returns all available template names.
var Names = []string{"default", "meeting", "brainstorm", "research"}

var bodies = map[string]string{
	"default": "",

	"meeting": `## {{title}}

**Date:** {{date}}
**Attendees:**

## Agenda

-

## Notes

## Action Items

- [ ]
`,

	"brainstorm": `## {{title}}

**Date:** {{date}}

## Core idea

## Branches

-
-
-

## Keep / discard

| Idea | Keep? |
|------|-------|
|      |       |
`,

	"research": `## {{title}}

**Date:** {{date}}

## Question

## Sources

-

## Notes

## Conclusion
`,
}

// Get returns the template body for the given name, with {{title}} and {{date}}
// replaced by the provided values.
// Unknown names fall back to the "default" template (empty body).
func Get(name, title, date string) string {
	body, ok := bodies[name]
	if !ok {
		body = bodies["default"]
	}
	body = strings.ReplaceAll(body, "{{title}}", title)
	body = strings.ReplaceAll(body, "{{date}}", date)
	return body
}
