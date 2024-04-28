package glow

func (n *Network) checkCycle(from, to string) bool {
	if from == to {
		return true
	}

	n.mu.RLock()
	defer n.mu.RUnlock()

	cycle := false
	dfs(n, from, func(node string) bool {
		cycle = node == to
		return !cycle
	})

	return cycle
}

func dfs(n *Network, root string, callback func(string) bool) {
	visited := make(map[string]bool)
	var stack []string
	stack = append(stack, root)

	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if !visited[node] {
			if !callback(node) {
				return
			}

			visited[node] = true

			for _, link := range n.ingress[node] {
				stack = append(stack, link.x)
			}
		}
	}
}
