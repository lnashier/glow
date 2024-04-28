package glow

import (
	"bytes"
	"text/template"
)

const tmpl = `strict digraph {
    node [shape=ellipse]

	{{ range .Nodes -}}
        "{{ . }}" [style="{{ prop "style" . }}", fillcolor="{{ prop "color" . }}"];
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

			// node with egress and distributor mode set
			if len(egress) > 0 && node.distributor {
				if prop == "color" {
					return "lightyellow"
				}
				if prop == "style" {
					return "filled"
				}
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

	_, err := t.Parse(tmpl)
	if err != nil {
		return nil, err
	}

	var tpl bytes.Buffer
	if err = t.Execute(&tpl, n); err != nil {
		return nil, err
	}
	return tpl.Bytes(), nil
}
