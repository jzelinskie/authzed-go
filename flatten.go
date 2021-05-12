package authzed

import (
	"fmt"

	api "github.com/authzed/authzed-go/arrakisapi/api"
)

// Flatten reduces an ExpandResponse into the slice of Users present in the
// expansion.
func Flatten(resp *api.ExpandResponse) []*api.User { return flatten(resp.TreeNode) }

func flatten(node *api.RelationTupleTreeNode) []*api.User {
	switch typed := node.NodeType.(type) {
	case *api.RelationTupleTreeNode_IntermediateNode:
		switch typed.IntermediateNode.Operation {
		case api.SetOperationUserset_UNION:
			return flattenUnion(typed.IntermediateNode.ChildNodes)
		case api.SetOperationUserset_INTERSECTION:
			return flattenIntersection(typed.IntermediateNode.ChildNodes)
		case api.SetOperationUserset_EXCLUSION:
			return flattenExclusion(typed.IntermediateNode.ChildNodes)
		}
	case *api.RelationTupleTreeNode_LeafNode:
		var toReturn userSet = make(map[string]struct{})
		for _, usr := range typed.LeafNode.Users {
			toReturn.add(usr)
		}
		return toReturn.toSlice()
	}
	return nil
}

func flattenUnion(children []*api.RelationTupleTreeNode) []*api.User {
	var toReturn userSet = make(map[string]struct{})
	for _, child := range children {
		toReturn.add(flatten(child)...)
	}
	return toReturn.toSlice()
}

func flattenIntersection(children []*api.RelationTupleTreeNode) []*api.User {
	firstChildChildren := flatten(children[0])

	if len(children) == 1 {
		return firstChildChildren
	}

	var inOthers userSet = make(map[string]struct{})
	inOthers.add(flattenIntersection(children[1:])...)

	maxChildren := len(firstChildChildren)
	if len(inOthers) < maxChildren {
		maxChildren = len(inOthers)
	}

	toReturn := make([]*api.User, 0, maxChildren)
	for _, child := range firstChildChildren {
		if inOthers.contains(child) {
			toReturn = append(toReturn, child)
		}
	}

	return toReturn
}

func flattenExclusion(children []*api.RelationTupleTreeNode) []*api.User {
	firstChildChildren := flatten(children[0])

	if len(children) == 1 || len(firstChildChildren) == 0 {
		return firstChildChildren
	}

	var toReturn userSet = make(map[string]struct{})
	toReturn.add(firstChildChildren...)
	for _, child := range children[1:] {
		toReturn.remove(flatten(child)...)
	}

	return toReturn.toSlice()
}

func leaf(start *api.ObjectAndRelation, children ...*api.User) *api.RelationTupleTreeNode {
	return &api.RelationTupleTreeNode{
		NodeType: &api.RelationTupleTreeNode_LeafNode{
			LeafNode: &api.DirectUserset{
				Users: children,
			},
		},
		Expanded: start,
	}
}

func setResult(
	op api.SetOperationUserset_Operation,
	start *api.ObjectAndRelation,
	children []*api.RelationTupleTreeNode,
) *api.RelationTupleTreeNode {
	return &api.RelationTupleTreeNode{
		NodeType: &api.RelationTupleTreeNode_IntermediateNode{
			IntermediateNode: &api.SetOperationUserset{
				Operation:  op,
				ChildNodes: children,
			},
		},
		Expanded: start,
	}
}

func union(start *api.ObjectAndRelation, children ...*api.RelationTupleTreeNode) *api.RelationTupleTreeNode {
	return setResult(api.SetOperationUserset_UNION, start, children)
}

func intersection(start *api.ObjectAndRelation, children ...*api.RelationTupleTreeNode) *api.RelationTupleTreeNode {
	return setResult(api.SetOperationUserset_INTERSECTION, start, children)
}

func exclusion(start *api.ObjectAndRelation, children ...*api.RelationTupleTreeNode) *api.RelationTupleTreeNode {
	return setResult(api.SetOperationUserset_EXCLUSION, start, children)
}

type userSet map[string]struct{}

func (us userSet) add(users ...*api.User) {
	for _, usr := range users {
		us[toKey(usr)] = struct{}{}
	}
}

func (us userSet) contains(usr *api.User) bool {
	_, ok := us[toKey(usr)]
	return ok
}

func (us userSet) remove(users ...*api.User) {
	for _, usr := range users {
		delete(us, toKey(usr))
	}
}

func (us userSet) toSlice() []*api.User {
	toReturn := make([]*api.User, 0, len(us))
	for key := range us {
		toReturn = append(toReturn, fromKey(key))
	}
	return toReturn
}

func toKey(usr *api.User) string {
	return fmt.Sprintf("%s %s %s", usr.GetUserset().Namespace, usr.GetUserset().ObjectId, usr.GetUserset().Relation)
}

func fromKey(key string) *api.User {
	userset := &api.ObjectAndRelation{}
	fmt.Sscanf(key, "%s %s %s", &userset.Namespace, &userset.ObjectId, &userset.Relation)
	return &api.User{
		UserOneof: &api.User_Userset{Userset: userset},
	}
}
