package dag

import (
	"errors"
	"sort"
)

// Graph adjacency list
type Graph struct {
	Nodes map[string][]string
}

func New() *Graph {
	return &Graph{Nodes: make(map[string][]string)}
}

func (g *Graph) AddEdge(from, to string) {
	g.Nodes[to] = append(g.Nodes[to], from)
}

// Sort topological order
func (g *Graph) Sort(items []string) ([]string, error) {
	visited := make(map[string]bool)
	tempMark := make(map[string]bool)
	var result []string

	var visit func(string) error
	visit = func(n string) error {
		if tempMark[n] {
			return errors.New("detected cycle in dependency graph")
		}
		if visited[n] {
			return nil
		}
		tempMark[n] = true

		for _, dep := range g.Nodes[n] {
			if err := visit(dep); err != nil {
				return err
			}
		}

		visited[n] = true
		tempMark[n] = false
		result = append(result, n)
		return nil
	}

	for _, item := range items {
		if !visited[item] {
			if err := visit(item); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func (g *Graph) SortLayers(items []string) ([][]string, error) {
	adj := make(map[string][]string)
	inDegree := make(map[string]int)

	for _, id := range items {
		inDegree[id] = 0
	}

	// Fill data based on dependencies
	// g.Nodes[child] = [parent1, parent2...]
	for _, child := range items {
		parents := g.Nodes[child]
		inDegree[child] = len(parents) // Node has N dependencies
		for _, p := range parents {
			// Record that Parent points to Child
			adj[p] = append(adj[p], child)
		}
	}

	var layers [][]string
	var queue []string

	for id, d := range inDegree {
		if d == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue) // Sort for deterministic behavior

	processedCount := 0

	for len(queue) > 0 {
		layers = append(layers, queue) // Add current layer
		processedCount += len(queue)
		
		var nextQueue []string

		// Process current layer
		for _, u := range queue {
			// 'u' is completed, now verify its children
			for _, v := range adj[u] {
				inDegree[v]--
				if inDegree[v] == 0 {
					nextQueue = append(nextQueue, v)
				}
			}
		}
		sort.Strings(nextQueue)
		queue = nextQueue
	}

	if processedCount != len(items) {
		return nil, errors.New("detected cycle in dependency graph")
	}

	return layers, nil
}
