package service

import (
	"slices"

	"github.com/optikklabs/query/internal/modules/traces/models"
	"github.com/optikklabs/query/internal/modules/traces/repository"
)

// buildCriticalPath walks the longest-duration parent->child chain:
// pick the root whose subtree ends last, then repeatedly descend into
// the child whose subtree ends last.
func buildCriticalPath(rows []repository.TraceSpanRow) []models.CriticalPathSpan {
	nodes, roots := indexNodes(rows)
	computeSubtreeEnds(nodes, roots)
	bestRoot := pickBestRoot(nodes, roots)
	return walkCriticalChain(nodes, bestRoot)
}

type criticalNode struct {
	row        *repository.TraceSpanRow
	startNs    int64
	subtreeEnd int64
	children   []string
}

func indexNodes(rows []repository.TraceSpanRow) (map[string]*criticalNode, []string) {
	nodes := make(map[string]*criticalNode, len(rows))
	var roots []string
	for i := range rows {
		row := &rows[i]
		startNs := row.Timestamp.UnixNano()
		nodes[row.SpanID] = &criticalNode{row: row, startNs: startNs, subtreeEnd: startNs + int64(row.DurationNano)}
		if isRootParentSpanID(row.ParentSpanID) {
			roots = append(roots, row.SpanID)
		}
	}
	for sid, n := range nodes {
		if !isRootParentSpanID(n.row.ParentSpanID) {
			if parent, ok := nodes[n.row.ParentSpanID]; ok {
				parent.children = append(parent.children, sid)
			}
		}
	}
	return nodes, roots
}

func computeSubtreeEnds(nodes map[string]*criticalNode, roots []string) {
	type frame struct {
		spanID   string
		childIdx int
	}
	for _, root := range roots {
		stack := []frame{{spanID: root}}
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			n := nodes[top.spanID]
			if top.childIdx < len(n.children) {
				cid := n.children[top.childIdx]
				top.childIdx++
				stack = append(stack, frame{spanID: cid})
			} else {
				for _, cid := range n.children {
					if child := nodes[cid]; child.subtreeEnd > n.subtreeEnd {
						n.subtreeEnd = child.subtreeEnd
					}
				}
				stack = stack[:len(stack)-1]
			}
		}
	}
}

func pickBestRoot(nodes map[string]*criticalNode, roots []string) string {
	var bestRoot string
	var bestEnd int64
	for _, root := range roots {
		if n := nodes[root]; n.subtreeEnd > bestEnd {
			bestEnd = n.subtreeEnd
			bestRoot = root
		}
	}
	return bestRoot
}

// walkCriticalChain follows the chain from root. A span's SelfMs is the
// stretch of the path's wall time it accounts for: from its start to where the
// path ends below it, minus the same stretch for its critical child. Measuring
// to subtree ends rather than span ends keeps async children that outlive
// their parent from being counted twice, so SelfMs sums to the path's span.
func walkCriticalChain(nodes map[string]*criticalNode, root string) []models.CriticalPathSpan {
	result := []models.CriticalPathSpan{}
	for cur := root; cur != ""; {
		n := nodes[cur]
		selfNs := n.subtreeEnd - n.startNs
		cur = ""
		if len(n.children) > 0 {
			cur = pickBestChild(nodes, n.children)
			child := nodes[cur]
			selfNs = max(0, selfNs-(child.subtreeEnd-child.startNs))
		}
		result = append(result, models.CriticalPathSpan{
			SpanID:        n.row.SpanID,
			OperationName: n.row.OperationName,
			ServiceName:   n.row.ServiceName,
			DurationMs:    n.row.DurationMs(),
			SelfMs:        float64(selfNs) / 1e6,
		})
	}
	return result
}

func pickBestChild(nodes map[string]*criticalNode, children []string) string {
	var best string
	var bestEnd, bestStart int64
	for _, cid := range children {
		child := nodes[cid]
		if child.subtreeEnd > bestEnd || (child.subtreeEnd == bestEnd && child.startNs > bestStart) {
			bestEnd = child.subtreeEnd
			bestStart = child.startNs
			best = cid
		}
	}
	return best
}

// buildErrorPath returns the root-to-leaf ancestry chain of error spans,
// ending at the earliest error span that no other error span has as parent.
func buildErrorPath(rows []repository.TraceSpanRow) []models.ErrorPathSpan {
	spans := make(map[string]*repository.TraceSpanRow, len(rows))
	for i := range rows {
		spans[rows[i].SpanID] = &rows[i]
	}
	leafID := pickErrorLeaf(rows)
	if leafID == "" {
		return []models.ErrorPathSpan{}
	}
	chain := walkErrorChain(spans, leafID)
	slices.Reverse(chain)
	return chain
}

// pickErrorLeaf returns the first span, in row (start time) order, that is
// not the parent of another span in rows.
func pickErrorLeaf(rows []repository.TraceSpanRow) string {
	isParent := make(map[string]bool, len(rows))
	for i := range rows {
		isParent[rows[i].ParentSpanID] = true
	}
	for i := range rows {
		if !isParent[rows[i].SpanID] {
			return rows[i].SpanID
		}
	}
	return ""
}

func walkErrorChain(spans map[string]*repository.TraceSpanRow, leafID string) []models.ErrorPathSpan {
	var chain []models.ErrorPathSpan
	cur := leafID
	for cur != "" {
		s, ok := spans[cur]
		if !ok {
			break
		}
		chain = append(chain, models.ErrorPathSpan{
			SpanID:        s.SpanID,
			ParentSpanID:  s.ParentSpanID,
			OperationName: s.OperationName,
			ServiceName:   s.ServiceName,
			Status:        s.StatusCode,
			StatusMessage: s.StatusMessage,
			StartTime:     s.Timestamp,
			DurationMs:    s.DurationMs(),
		})
		cur = s.ParentSpanID
	}
	return chain
}

// isRootParentSpanID reports a root span: ingest stores an absent (or
// all-zero) parent span id as "".
func isRootParentSpanID(parentID string) bool {
	return parentID == ""
}
