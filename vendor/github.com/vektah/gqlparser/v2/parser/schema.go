package parser

import (
	. "github.com/vektah/gqlparser/v2/ast" //nolint:staticcheck // bad, yeah
	"github.com/vektah/gqlparser/v2/lexer"
)

func ParseSchemas(inputs ...*Source) (*SchemaDocument, error) {
	sd := &SchemaDocument{}
	for _, input := range inputs {
		inputAst, err := ParseSchema(input)
		if err != nil {
			return nil, err
		}
		sd.Merge(inputAst)
	}
	return sd, nil
}

func ParseSchema(source *Source) (*SchemaDocument, error) {
	p := parser{
		lexer:         lexer.New(source),
		maxTokenLimit: 0, // default value is unlimited
	}
	sd, err := p.parseSchemaDocument(), p.err
	if err != nil {
		return nil, err
	}

	for _, def := range sd.Definitions {
		def.BuiltIn = source.BuiltIn
	}
	for _, def := range sd.Extensions {
		def.BuiltIn = source.BuiltIn
	}

	return sd, nil
}

func ParseSchemasWithLimit(maxTokenLimit int, inputs ...*Source) (*SchemaDocument, error) {
	sd := &SchemaDocument{}
	for _, input := range inputs {
		inputAst, err := ParseSchemaWithLimit(input, maxTokenLimit)
		if err != nil {
			return nil, err
		}
		sd.Merge(inputAst)
	}
	return sd, nil
}

func ParseSchemaWithLimit(source *Source, maxTokenLimit int) (*SchemaDocument, error) {
	p := parser{
		lexer:         lexer.New(source),
		maxTokenLimit: maxTokenLimit, // 0 is unlimited
	}
	sd, err := p.parseSchemaDocument(), p.err
	if err != nil {
		return nil, err
	}

	for _, def := range sd.Definitions {
		def.BuiltIn = source.BuiltIn
	}
	for _, def := range sd.Extensions {
		def.BuiltIn = source.BuiltIn
	}

	return sd, nil
}

func (p *parser) parseSchemaDocument() *SchemaDocument {
	var doc SchemaDocument
	doc.Position = p.peekPos()
	for p.peek().Kind != lexer.EOF {
		if p.err != nil {
			return nil
		}

		var description descriptionWithComment
		if p.peek().Kind == lexer.BlockString || p.peek().Kind == lexer.String {
			description = p.parseDescription()
		}

		if p.peek().Kind != lexer.Name {
			p.unexpectedError()
			break
		}

		switch p.peek().Value {
		case "scalar", "type", "interface", "union", "enum", "input":
			doc.Definitions = append(doc.Definitions, p.parseTypeSystemDefinition(description))
		case "schema":
			doc.Schema = append(doc.Schema, p.parseSchemaDefinition(description))
		case "directive":
			doc.Directives = append(doc.Directives, p.parseDirectiveDefinition(description))
		case "extend":
			if description.text != "" {
				p.unexpectedToken(p.prev)
			}
			p.parseTypeSystemExtension(&doc)
		default:
			p.unexpectedError()
			return nil
		}
	}

	// treat end of file comments
	doc.Comment = p.comment

	return &doc
}

func (p *parser) parseDescription() descriptionWithComment {
	token := p.peek()

	var desc descriptionWithComment
	if token.Kind != lexer.BlockString && token.Kind != lexer.String {
		return desc
	}

	desc.comment = p.comment
	desc.text = p.next().Value
	return desc
}

func (p *parser) parseTypeSystemDefinition(description descriptionWithComment) *Definition {
	tok := p.peek()
	if tok.Kind != lexer.Name {
		p.unexpectedError()
		return nil
	}

	switch tok.Value {
	case "scalar":
		return p.parseScalarTypeDefinition(description)
	case "type":
		return p.parseObjectTypeDefinition(description)
	case "interface":
		return p.parseInterfaceTypeDefinition(description)
	case "union":
		return p.parseUnionTypeDefinition(description)
	case "enum":
		return p.parseEnumTypeDefinition(description)
	case "input":
		return p.parseInputObjectTypeDefinition(description)
	default:
		p.unexpectedError()
		return nil
	}
}

func (p *parser) parseSchemaDefinition(description descriptionWithComment) *SchemaDefinition {
	_, comment := p.expectKeyword("schema")

	def := SchemaDefinition{}
	def.Position = p.peekPos()
	def.BeforeDescriptionComment = description.comment
	def.Description = description.text
	def.AfterDescriptionComment = comment
	def.Directives = p.parseDirectives(true)

	def.EndOfDefinitionComment = p.some(lexer.BraceL, lexer.BraceR, func() {
		def.OperationTypes = append(def.OperationTypes, p.parseOperationTypeDefinition())
	})
	return &def
}

func (p *parser) parseOperationTypeDefinition() *OperationTypeDefinition {
	var op OperationTypeDefinition
	op.Position = p.peekPos()
	op.Comment = p.comment
	op.Operation = p.parseOperationType()
	p.expect(lexer.Colon)
	op.Type = p.parseName()
	return &op
}

func (p *parser) parseScalarTypeDefinition(description descriptionWithComment) *Definition {
	_, comment := p.expectKeyword("scalar")

	var def Definition
	def.Position = p.peekPos()
	def.BeforeDescriptionComment = description.comment
	def.Description = description.text
	def.AfterDescriptionComment = comment
	def.Kind = Scalar
	def.Name = p.parseName()
	def.Directives = p.parseDirectives(true)
	return &def
}

func (p *parser) parseObjectTypeDefinition(description descriptionWithComment) *Definition {
	_, comment := p.expectKeyword("type")

	var def Definition
	def.Position = p.peekPos()
	def.Kind = Object
	def.BeforeDescriptionComment = description.comment
	def.Description = description.text
	def.AfterDescriptionComment = comment
	def.Name = p.parseName()
	def.Interfaces = p.parseImplementsInterfaces()
	def.Directives = p.parseDirectives(true)
	def.Fields, def.EndOfDefinitionComment = p.parseFieldsDefinition()
	return &def
}

func (p *parser) parseImplementsInterfaces() []string {
	var types []string
	if p.peek().Value == "implements" {
		p.next()
		// optional leading ampersand
		p.skip(lexer.Amp)

		types = append(types, p.parseName())
		for p.skip(lexer.Amp) && p.err == nil {
			types = append(types, p.parseName())
		}
	}
	return types
}

func (p *parser) parseFieldsDefinition() (FieldList, *CommentGroup) {
	var defs FieldList
	comment := p.some(lexer.BraceL, lexer.BraceR, func() {
		defs = append(defs, p.parseFieldDefinition())
	})
	return defs, comment
}

func (p *parser) parseFieldDefinition() *FieldDefinition {
	var def FieldDefinition
	def.Position = p.peekPos()

	desc := p.parseDescription()
	if desc.text != "" {
		def.BeforeDescriptionComment = desc.comment
		def.Description = desc.text
	}

	p.peek() // peek to set p.comment
	def.AfterDescriptionComment = p.comment
	def.Name = p.parseName()
	def.Arguments = p.parseArgumentDefs()
	p.expect(lexer.Colon)
	def.Type = p.parseTypeReference()
	def.Directives = p.parseDirectives(true)

	return &def
}

func (p *parser) parseArgumentDefs() ArgumentDefinitionList {
	var args ArgumentDefinitionList
	p.some(lexer.ParenL, lexer.ParenR, func() {
		args = append(args, p.parseArgumentDef())
	})
	return args
}

func (p *parser) parseArgumentDef() *ArgumentDefinition {
	var def ArgumentDefinition
	def.Position = p.peekPos()

	desc := p.parseDescription()
	if desc.text != "" {
		def.BeforeDescriptionComment = desc.comment
		def.Description = desc.text
	}

	p.peek() // peek to set p.comment
	def.AfterDescriptionComment = p.comment
	def.Name = p.parseName()
	p.expect(lexer.Colon)
	def.Type = p.parseTypeReference()
	if p.skip(lexer.Equals) {
		def.DefaultValue = p.parseValueLiteral(true)
	}
	def.Directives = p.parseDirectives(true)
	return &def
}

func (p *parser) parseInputValueDef() *FieldDefinition {
	var def FieldDefinition
	def.Position = p.peekPos()

	desc := p.parseDescription()
	if desc.text != "" {
		def.BeforeDescriptionComment = desc.comment
		def.Description = desc.text
	}

	p.peek() // peek to set p.comment
	def.AfterDescriptionComment = p.comment
	def.Name = p.parseName()
	p.expect(lexer.Colon)
	def.Type = p.parseTypeReference()
	if p.skip(lexer.Equals) {
		def.DefaultValue = p.parseValueLiteral(true)
	}
	def.Directives = p.parseDirectives(true)
	return &def
}

func (p *parser) parseInterfaceTypeDefinition(description descriptionWithComment) *Definition {
	_, comment := p.expectKeyword("interface")

	var def Definition
	def.Position = p.peekPos()
	def.Kind = Interface
	def.BeforeDescriptionComment = description.comment
	def.Description = description.text
	def.AfterDescriptionComment = comment
	def.Name = p.parseName()
	def.Interfaces = p.parseImplementsInterfaces()
	def.Directives = p.parseDirectives(true)
	def.Fields, def.EndOfDefinitionComment = p.parseFieldsDefinition()
	return &def
}

func (p *parser) parseUnionTypeDefinition(description descriptionWithComment) *Definition {
	_, comment := p.expectKeyword("union")

	var def Definition
	def.Position = p.peekPos()
	def.Kind = Union
	def.BeforeDescriptionComment = description.comment
	def.Description = description.text
	def.AfterDescriptionComment = comment
	def.Name = p.parseName()
	def.Directives = p.parseDirectives(true)
	def.Types = p.parseUnionMemberTypes()
	return &def
}

func (p *parser) parseUnionMemberTypes() []string {
	var types []string
	if p.skip(lexer.Equals) {
		// optional leading pipe
		p.skip(lexer.Pipe)

		types = append(types, p.parseName())
		for p.skip(lexer.Pipe) && p.err == nil {
			types = append(types, p.parseName())
		}
	}
	return types
}

func (p *parser) parseEnumTypeDefinition(description descriptionWithComment) *Definition {
	_, comment := p.expectKeyword("enum")

	var def Definition
	def.Position = p.peekPos()
	def.Kind = Enum
	def.BeforeDescriptionComment = description.comment
	def.Description = description.text
	def.AfterDescriptionComment = comment
	def.Name = p.parseName()
	def.Directives = p.parseDirectives(true)
	def.EnumValues, def.EndOfDefinitionComment = p.parseEnumValuesDefinition()
	return &def
}

func (p *parser) parseEnumValuesDefinition() (EnumValueList, *CommentGroup) {
	var values EnumValueList
	comment := p.some(lexer.BraceL, lexer.BraceR, func() {
		values = append(values, p.parseEnumValueDefinition())
	})
	return values, comment
}

func (p *parser) parseEnumValueDefinition() *EnumValueDefinition {
	var def EnumValueDefinition
	def.Position = p.peekPos()
	desc := p.parseDescription()
	if desc.text != "" {
		def.BeforeDescriptionComment = desc.comment
		def.Description = desc.text
	}

	p.peek() // peek to set p.comment
	def.AfterDescriptionComment = p.comment

	def.Name = p.parseName()
	def.Directives = p.parseDirectives(true)

	return &def
}

func (p *parser) parseInputObjectTypeDefinition(description descriptionWithComment) *Definition {
	_, comment := p.expectKeyword("input")

	var def Definition
	def.Position = p.peekPos()
	def.Kind = InputObject
	def.BeforeDescriptionComment = description.comment
	def.Description = description.text
	def.AfterDescriptionComment = comment
	def.Name = p.parseName()
	def.Directives = p.parseDirectives(true)
	def.Fields, def.EndOfDefinitionComment = p.parseInputFieldsDefinition()
	return &def
}

func (p *parser) parseInputFieldsDefinition() (FieldList, *CommentGroup) {
	var values FieldList
	comment := p.some(lexer.BraceL, lexer.BraceR, func() {
		values = append(values, p.parseInputValueDef())
	})
	return values, comment
}

func (p *parser) parseTypeSystemExtension(doc *SchemaDocument) {
	_, comment := p.expectKeyword("extend")

	switch p.peek().Value {
	case "schema":
		doc.SchemaExtension = append(doc.SchemaExtension, p.parseSchemaExtension(comment))
	case "scalar":
		doc.Extensions = append(doc.Extensions, p.parseScalarTypeExtension(comment))
	case "type":
		doc.Extensions = append(doc.Extensions, p.parseObjectTypeExtension(comment))
	case "interface":
		doc.Extensions = append(doc.Extensions, p.parseInterfaceTypeExtension(comment))
	case "union":
		doc.Extensions = append(doc.Extensions, p.parseUnionTypeExtension(comment))
	case "enum":
		doc.Extensions = append(doc.Extensions, p.parseEnumTypeExtension(comment))
	case "input":
		doc.Extensions = append(doc.Extensions, p.parseInputObjectTypeExtension(comment))
	default:
		p.unexpectedError()
	}
}

func (p *parser) parseSchemaExtension(comment *CommentGroup) *SchemaDefinition {
	p.expectKeyword("schema")

	var def SchemaDefinition
	def.Position = p.peekPos()
	def.AfterDescriptionComment = comment
	def.Directives = p.parseDirectives(true)
	def.EndOfDefinitionComment = p.some(lexer.BraceL, lexer.BraceR, func() {
		def.OperationTypes = append(def.OperationTypes, p.parseOperationTypeDefinition())
	})
	if len(def.Directives) == 0 && len(def.OperationTypes) == 0 {
		p.unexpectedError()
	}
	return &def
}

func (p *parser) parseScalarTypeExtension(comment *CommentGroup) *Definition {
	p.expectKeyword("scalar")

	var def Definition
	def.Position = p.peekPos()
	def.AfterDescriptionComment = comment
	def.Kind = Scalar
	def.Name = p.parseName()
	def.Directives = p.parseDirectives(true)
	if len(def.Directives) == 0 {
		p.unexpectedError()
	}
	return &def
}

func (p *parser) parseObjectTypeExtension(comment *CommentGroup) *Definition {
	p.expectKeyword("type")

	var def Definition
	def.Position = p.peekPos()
	def.AfterDescriptionComment = comment
	def.Kind = Object
	def.Name = p.parseName()
	def.Interfaces = p.parseImplementsInterfaces()
	def.Directives = p.parseDirectives(true)
	def.Fields, def.EndOfDefinitionComment = p.parseFieldsDefinition()
	if len(def.Interfaces) == 0 && len(def.Directives) == 0 && len(def.Fields) == 0 {
		p.unexpectedError()
	}
	return &def
}

func (p *parser) parseInterfaceTypeExtension(comment *CommentGroup) *Definition {
	p.expectKeyword("interface")

	var def Definition
	def.Position = p.peekPos()
	def.AfterDescriptionComment = comment
	def.Kind = Interface
	def.Name = p.parseName()
	def.Directives = p.parseDirectives(true)
	def.Fields, def.EndOfDefinitionComment = p.parseFieldsDefinition()
	if len(def.Directives) == 0 && len(def.Fields) == 0 {
		p.unexpectedError()
	}
	return &def
}

func (p *parser) parseUnionTypeExtension(comment *CommentGroup) *Definition {
	p.expectKeyword("union")

	var def Definition
	def.Position = p.peekPos()
	def.AfterDescriptionComment = comment
	def.Kind = Union
	def.Name = p.parseName()
	def.Directives = p.parseDirectives(true)
	def.Types = p.parseUnionMemberTypes()

	if len(def.Directives) == 0 && len(def.Types) == 0 {
		p.unexpectedError()
	}
	return &def
}

func (p *parser) parseEnumTypeExtension(comment *CommentGroup) *Definition {
	p.expectKeyword("enum")

	var def Definition
	def.Position = p.peekPos()
	def.AfterDescriptionComment = comment
	def.Kind = Enum
	def.Name = p.parseName()
	def.Directives = p.parseDirectives(true)
	def.EnumValues, def.EndOfDefinitionComment = p.parseEnumValuesDefinition()
	if len(def.Directives) == 0 && len(def.EnumValues) == 0 {
		p.unexpectedError()
	}
	return &def
}

func (p *parser) parseInputObjectTypeExtension(comment *CommentGroup) *Definition {
	p.expectKeyword("input")

	var def Definition
	def.Position = p.peekPos()
	def.AfterDescriptionComment = comment
	def.Kind = InputObject
	def.Name = p.parseName()
	def.Directives = p.parseDirectives(false)
	def.Fields, def.EndOfDefinitionComment = p.parseInputFieldsDefinition()
	if len(def.Directives) == 0 && len(def.Fields) == 0 {
		p.unexpectedError()
	}
	return &def
}

func (p *parser) parseDirectiveDefinition(description descriptionWithComment) *DirectiveDefinition {
	_, comment := p.expectKeyword("directive")
	p.expect(lexer.At)

	var def DirectiveDefinition
	def.Position = p.peekPos()
	def.BeforeDescriptionComment = description.comment
	def.Description = description.text
	def.AfterDescriptionComment = comment
	def.Name = p.parseName()
	def.Arguments = p.parseArgumentDefs()

	if peek := p.peek(); peek.Kind == lexer.Name && peek.Value == "repeatable" {
		def.IsRepeatable = true
		p.skip(lexer.Name)
	}

	p.expectKeyword("on")
	def.Locations = p.parseDirectiveLocations()
	return &def
}

func (p *parser) parseDirectiveLocations() []DirectiveLocation {
	p.skip(lexer.Pipe)

	locations := []DirectiveLocation{p.parseDirectiveLocation()}

	for p.skip(lexer.Pipe) && p.err == nil {
		locations = append(locations, p.parseDirectiveLocation())
	}

	return locations
}

func (p *parser) parseDirectiveLocation() DirectiveLocation {
	name, _ := p.expect(lexer.Name)

	switch name.Value {
	case `QUERY`:
		return LocationQuery
	case `MUTATION`:
		return LocationMutation
	case `SUBSCRIPTION`:
		return LocationSubscription
	case `FIELD`:
		return LocationField
	case `FRAGMENT_DEFINITION`:
		return LocationFragmentDefinition
	case `FRAGMENT_SPREAD`:
		return LocationFragmentSpread
	case `INLINE_FRAGMENT`:
		return LocationInlineFragment
	case `VARIABLE_DEFINITION`:
		return LocationVariableDefinition
	case `SCHEMA`:
		return LocationSchema
	case `SCALAR`:
		return LocationScalar
	case `OBJECT`:
		return LocationObject
	case `FIELD_DEFINITION`:
		return LocationFieldDefinition
	case `ARGUMENT_DEFINITION`:
		return LocationArgumentDefinition
	case `INTERFACE`:
		return LocationInterface
	case `UNION`:
		return LocationUnion
	case `ENUM`:
		return LocationEnum
	case `ENUM_VALUE`:
		return LocationEnumValue
	case `INPUT_OBJECT`:
		return LocationInputObject
	case `INPUT_FIELD_DEFINITION`:
		return LocationInputFieldDefinition
	}

	p.unexpectedToken(name)
	return ""
}

type descriptionWithComment struct {
	text    string
	comment *CommentGroup
}
