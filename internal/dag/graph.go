package dag

import (
	"errors"
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


