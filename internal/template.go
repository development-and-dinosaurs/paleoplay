package internal

// Scalars that come from Steam are quoted rather than emitted bare: game names,
// franchises and tags routinely contain colons ("Teenage Mutant Ninja Turtles:
// Splintered Fate"), which YAML reads as a second mapping value and rejects.
// Franchise is left bare when empty so it stays null rather than becoming an
// empty taxonomy term.
var postTemplate string = `---
title: |-
  {{ .Date }} - {{ .Game }}
dates: {{ .Date }}
images:
{{- range .Images }}
  - {{ printf "%q" . }}
{{- end }}
games: |-
   {{ .Game }}
price: {{ .Price }}
genres:
{{- range .Genres }}
  - {{ printf "%q" . }}
{{- end }}
developers:
{{- range .Developers }}
  - {{ printf "%q" . }}
{{- end }}
publishers:
{{- range .Publishers }}
  - {{ printf "%q" . }}
{{- end }}
franchises:{{ if .Franchise }} {{ printf "%q" .Franchise }}{{ end }}
tags:
{{- range .Tags }}
  - {{ printf "%q" . }}
{{- end }}
steamId: {{ .SteamId }}
paleoplay: {{ .Paleoplay }}
---
`
