package parsers

type sourceContext struct {
	doc        *ASTDocument
	blockIndex int // Index of the block in doc.Nodes
}
