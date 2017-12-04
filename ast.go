package graphqlws

import (
	"github.com/graphql-go/graphql/language/ast"
)

func operationDefinitionsWithOperation(
	doc *ast.Document,
	op string,
) []*ast.OperationDefinition {
	defs := []*ast.OperationDefinition{}
	for _, node := range doc.Definitions {
		if node.GetKind() == "OperationDefinition" {
			if def, ok := node.(*ast.OperationDefinition); ok {
				defs = append(defs, def)
			}
		}
	}
	return defs
}

func selectionSetsForOperationDefinitions(
	defs []*ast.OperationDefinition,
) []*ast.SelectionSet {
	sets := []*ast.SelectionSet{}
	for _, def := range defs {
		if set := def.GetSelectionSet(); set != nil {
			sets = append(sets, set)
		}
	}
	return sets
}

func nameForSelectionSet(set *ast.SelectionSet) (string, bool) {
	if len(set.Selections) >= 1 {
		if field, ok := set.Selections[0].(*ast.Field); ok {
			return field.Name.Value, true
		}
	}
	return "", false
}

func namesForSelectionSets(sets []*ast.SelectionSet) []string {
	names := []string{}
	for _, set := range sets {
		if name, ok := nameForSelectionSet(set); ok {
			names = append(names, name)
		}
	}
	return names
}

func subscriptionFieldNamesFromDocument(doc *ast.Document) []string {
	defs := operationDefinitionsWithOperation(doc, "subscription")
	sets := selectionSetsForOperationDefinitions(defs)
	return namesForSelectionSets(sets)
}
