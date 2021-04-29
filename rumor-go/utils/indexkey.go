package utils

// key schemes
// primary document: {entityName}#{pk}
// indexed document: {entityName}@{indexName}:{indexValue}#pk
var DocumentSeqDelimiter = []byte(string('#'))
var DocumentIndexKeyDelimiter = []byte(string('@'))
var DocumentIndexValueDelimiter = []byte(string(':'))

func BuildDocumentKey(entityName, pk []byte) []byte {
	return ConcatBytes(
		entityName,
		DocumentSeqDelimiter,
		pk,
	)
}

func BuildDocumentGroupPrefix(entityName []byte) []byte {
	return ConcatBytes(
		entityName,
		DocumentSeqDelimiter,
	)
}

func BuildIndexGroupPrefix(entityName, indexName []byte) []byte {
	return ConcatBytes(
		entityName,
		DocumentIndexKeyDelimiter,
		indexName,
		DocumentIndexValueDelimiter,
	)
}

func GetReverseSeekKeyFromIndexGroupPrefix(key []byte) []byte {
	s := make([]byte, len(key))
	copy(s, key)
	s[len(s)-1] = s[len(s)-1] + 1
	return s
}

func BuildIndexIteratorPrefix(entityName, indexName, indexKey []byte) []byte {
	return ConcatBytes(
		entityName,
		DocumentIndexKeyDelimiter,
		indexName,
		DocumentIndexValueDelimiter,
		indexKey,
		DocumentSeqDelimiter,
	)
}

func BuildIndexedDocumentKey(entityName, indexName, indexKey, pk []byte) []byte {
	return ConcatBytes(
		BuildIndexIteratorPrefix(entityName, indexName, indexKey),
		pk,
	)
}
