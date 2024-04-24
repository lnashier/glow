package glow

import (
	"bytes"
	"slices"
	"text/template"
)

const dotTmpl = `strict digraph {
    node [shape=ellipse]
	{{ range $n := .Nodes -}}
        "{{ $n }}" [style="{{ style $n }}", fillcolor="{{ color $n }}"];
    {{ end -}}
    {{ range .Links -}}
        "{{ from . }}" -> "{{ to . }}";
    {{ end }}
}`

// DOT describes the Network.
func DOT(n *Network) ([]byte, error) {
	t := template.New("tmpl")
	t.Funcs(template.FuncMap{
		"color": func(k string) string {
			node, _ := n.Node(k)
			if node.distributor {
				return "lightyellow"
			}
			return "lightgreen"
		},
		"style": func(k string) string {
			if !slices.Contains(n.Seeds(), k) && !slices.Contains(n.Termini(), k) {
				return "filled"
			}
			return ""
		},
		"from": func(l *Link) string {
			return l.x
		},
		"to": func(l *Link) string {
			return l.y
		},
	})

	_, err := t.Parse(dotTmpl)
	if err != nil {
		return nil, err
	}

	var tpl bytes.Buffer
	if err := t.Execute(&tpl, n); err != nil {
		return nil, err
	}
	return tpl.Bytes(), nil
}
