package models

type ArchetypeSource string

const (
	ArchetypeSourceEmbed    ArchetypeSource = "embed"
	ArchetypeSourceExternal ArchetypeSource = "external"
)

type ArchetypeInfo struct {
	// Slug, archetype file name without extension
	Name string
	// Human friendly name for the archetype
	Label       string
	Description string
	Source      ArchetypeSource
}
