package install

import (
	"fmt"
	"regexp"
	"strings"
)

var parameterRe = regexp.MustCompile(`{[^}]*}`)
var pgURLRe = regexp.MustCompile(`{pgurl(:[-,0-9]+)?}`)
var pgPortRe = regexp.MustCompile(`{pgport(:[-,0-9]+)?}`)
var storeDirRe = regexp.MustCompile(`{store-dir}`)

type expander struct {
	node    int
	pgURLs  map[int]string
	pgPorts map[int]string
}

func (e *expander) maybeExpandMap(c *SyncedCluster, m map[int]string, nodeSpec string) (string, bool) {
	if nodeSpec == "" {
		nodeSpec = "all"
	} else {
		nodeSpec = nodeSpec[1:]
	}

	nodes, err := ListNodes(nodeSpec, len(c.VMs))
	if err != nil {
		return err.Error(), true
	}

	var result []string
	for _, i := range nodes {
		if s, ok := m[i]; ok {
			result = append(result, s)
		}
	}
	return strings.Join(result, " "), true
}

func (e *expander) maybeExpandPgURL(c *SyncedCluster, s string) (string, bool) {
	m := pgURLRe.FindStringSubmatch(s)
	if m == nil {
		return s, false
	}

	if e.pgURLs == nil {
		e.pgURLs = c.pgurls(allNodes(len(c.VMs)))
	}

	return e.maybeExpandMap(c, e.pgURLs, m[1])
}

func (e *expander) maybeExpandPgPort(c *SyncedCluster, s string) (string, bool) {
	m := pgPortRe.FindStringSubmatch(s)
	if m == nil {
		return s, false
	}

	if e.pgPorts == nil {
		e.pgPorts = make(map[int]string, len(c.VMs))
		for _, i := range allNodes(len(c.VMs)) {
			e.pgPorts[i] = fmt.Sprint(c.Impl.NodePort(c, i))
		}
	}

	return e.maybeExpandMap(c, e.pgPorts, m[1])
}

func (e *expander) maybeExpandStoreDir(c *SyncedCluster, s string) (string, bool) {
	if !storeDirRe.MatchString(s) {
		return s, false
	}
	return c.Impl.NodeDir(c, e.node), true
}

func (e *expander) expand(c *SyncedCluster, arg string) string {
	return parameterRe.ReplaceAllStringFunc(arg, func(s string) string {
		type expanderFunc func(*SyncedCluster, string) (string, bool)
		expanders := []expanderFunc{
			e.maybeExpandPgURL,
			e.maybeExpandPgPort,
			e.maybeExpandStoreDir,
		}
		for _, f := range expanders {
			s, expanded := f(c, s)
			if expanded {
				return s
			}
		}
		return s
	})
}
