package internal

var postTemplate string = `---
title: |-
  {{ .Date }} - {{ .Game }}
dates: {{ .Date }}
images: 
{{- range .Images }}
  - {{ . }}
{{- end }}
games: |-
   {{ .Game }}
price: {{ .Price }}
genres:
{{- range .Genres }}
  - {{ . }}
{{- end }}
developers:
{{- range .Developers }}
  - {{ . }}
{{- end }}
publishers:
{{- range .Publishers }}
  - {{ . }}
{{- end }}
franchises: {{ .Franchise }}
tags:
{{- range .Tags }}
  - {{.}}
{{- end }}
steamId: {{ .SteamId }}
paleoplay: {{ .Paleoplay }}
---
`
