package glow

import (
	"bytes"
	"text/template"
)

const dotTmpl = `strict digraph {
    {{ range $n := .Nodes -}}
        "{{ $n }}";
    {{ end -}}
    {{ range .Links -}}
        "{{ .X }}" -> "{{ .Y }}";
    {{ end }}
}`

// DOT describes the Network.
func DOT(n *Network) ([]byte, error) {
	return generate(n, dotTmpl)
}

func generate(n *Network, tmpl string) ([]byte, error) {
	t, err := template.New("tmpl").Parse(tmpl)
	if err != nil {
		return nil, err
	}
	var tpl bytes.Buffer
	if err := t.Execute(&tpl, n); err != nil {
		return nil, err
	}
	return tpl.Bytes(), nil
}
