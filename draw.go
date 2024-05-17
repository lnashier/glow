package glow

import (
	"bytes"
	"fmt"
	"text/template"
)

const tmpl = `strict digraph {
  	labelloc="t"
	label="{{ netProp "label" }}"

    node [shape=ellipse]
	{{ range .Nodes -}}
		"{{ .Key }}"
		[
			label="{{ nodeProp "label" . }}",
			style="{{ nodeProp "style" . }}",
			fillcolor="{{ nodeProp "color" . }}"
		];
    {{ end -}}
    {{ range .Links -}}
        "{{ .From.Key }}" -> "{{ .To.Key }}"
		[
			label="{{ linkProp "label" . }}",
			color="{{ linkProp "color" . }}"
			arrowhead="{{ linkProp "arrowhead" . }}"
		];
    {{ end }}
}`

// DOT describes the Network.
func DOT(n *Network) ([]byte, error) {
	t := template.New("tmpl")
	t.Funcs(template.FuncMap{
		"netProp": func(prop string) any {
			switch prop {
			case "label":
				return fmt.Sprintf("Network Uptime: %s\n", n.Uptime())
			default:
				return ""
			}
		},
		"nodeProp": func(prop string, node *Node) any {
			switch prop {
			case "label":
				return fmt.Sprintf("%s\n(%s)", node.key, node.Uptime())
			case "color":
				switch {
				case len(n.Egress(node.Key())) > 0 && node.distributor:
					// node with egress and distributor mode set
					return "lightyellow"
				default:
					return ""
				}
			case "style":
				switch {
				case len(n.Egress(node.Key())) > 0 && node.distributor:
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
			case "label":
				return fmt.Sprintf("%d\n  (%s)", link.Tally(), link.Uptime())
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
