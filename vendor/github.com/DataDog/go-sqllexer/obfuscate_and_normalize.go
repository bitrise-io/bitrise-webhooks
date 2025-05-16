package sqllexer

import "strings"

// ObfuscateAndNormalize takes an input SQL string and returns an normalized SQL string with metadata
// This function is a convenience function that combines the Obfuscator and Normalizer in one pass
func ObfuscateAndNormalize(input string, obfuscator *Obfuscator, normalizer *Normalizer, lexerOpts ...lexerOption) (normalizedSQL string, statementMetadata *StatementMetadata, err error) {
	lexer := New(input, lexerOpts...)
	normalizedSQLBuilder := new(strings.Builder)
	normalizedSQLBuilder.Grow(len(input))

	// Always allocate metadata for backward compatibility
	statementMetadata = statementMetadataPool.Get().(*StatementMetadata)
	statementMetadata.reset()
	defer statementMetadataPool.Put(statementMetadata)

	obfuscate := func(token *Token, lastValueToken *LastValueToken) {
		obfuscator.ObfuscateTokenValue(token, lastValueToken, lexerOpts...)
	}

	// Pass obfuscation as the pre-process step
	if err = normalizer.normalizeToken(lexer, normalizedSQLBuilder, statementMetadata, obfuscate, lexerOpts...); err != nil {
		return "", nil, err
	}

	normalizedSQL = normalizedSQLBuilder.String()
	return normalizer.trimNormalizedSQL(normalizedSQL), statementMetadata, nil
}
