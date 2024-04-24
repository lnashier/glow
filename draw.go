package glow

import (
	"bytes"
	"slices"
	"text/template"
)

const dotTmpl = `strict digraph {
	{{ range $n := .Nodes -}}
        "{{ $n }}" [shape="{{ prop "shape" $n }}" style="{{ prop "style" $n }}", fillcolor="{{ prop "color" $n }}"];
    {{ end -}}
    {{ range .Links -}}
        "{{ from . }}" -> "{{ to . }}";
    {{ end }}
}`

// DOT describes the Network.
func DOT(n *Network) ([]byte, error) {
	t := template.New("tmpl")
	t.Funcs(template.FuncMap{
		"prop": func(prop string, k string) string {
			node, _ := n.Node(k)
			egress := n.Egress(k)

			switch {
			// node with egress and distributor mode set
			case len(egress) > 0 && node.distributor:
				if prop == "color" {
					return "lightyellow"
				}
				if prop == "style" {
					return "filled"
				}
			// any node with egress is broadcaster node if not distributor
			case len(egress) > 0:
				if prop == "color" {
					return "lightgreen"
				}
				if prop == "style" {
					return "filled"
				}
			}

			if prop == "shape" {
				if slices.Contains(n.Seeds(), k) {
					return "circle"
				}
				return "ellipse"
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
