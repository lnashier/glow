package glow

import (
	"bytes"
	"text/template"
)

const tmpl = `strict digraph {
    node [shape=ellipse]
	{{ range .Nodes -}}
        "{{ . }}"
		[
			style="{{ nodeProp "style" . }}",
			fillcolor="{{ nodeProp "color" . }}"
		];
    {{ end -}}
    {{ range .Links -}}
        "{{ .From }}" -> "{{ .To }}"
		[
			label="  {{ .Tally }}",
			color="{{ linkProp "color" . }}"
			arrowhead="{{ linkProp "arrowhead" . }}"
		];
    {{ end }}
}`

// DOT describes the Network.
func DOT(n *Network) ([]byte, error) {
	t := template.New("tmpl")
	t.Funcs(template.FuncMap{
		"nodeProp": func(prop string, k string) any {
			node, _ := n.Node(k)
			egress := n.Egress(k)

			switch prop {
			case "color":
				switch {
				case len(egress) > 0 && node.distributor:
					// node with egress and distributor mode set
					return "lightyellow"
				default:
					return ""
				}
			case "style":
				switch {
				case len(egress) > 0 && node.distributor:
					// node with egress and distributor mode set
					return "filled"
				default:
					return ""
				}
			default:
				return ""
			}
		},
		"linkProp": func(prop string, link *Link) any {
			switch prop {
			case "color":
				switch {
				case link.paused:
					return "gray"
				case link.deleted:
					return "red"
				default:
					return "lightblue"
				}
			case "arrowhead":
				switch {
				case link.paused || link.deleted:
					return "none"
				default:
					return "normal"
				}
			//case "penwidth":
			default:
				return ""
			}
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
