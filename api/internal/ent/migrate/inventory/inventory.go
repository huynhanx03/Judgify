// Package inventory inspects checked-in SQL migrations against live Ent schema
// declarations without connecting to a database.
package inventory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entfield "entgo.io/ent/schema/field"
	"github.com/go-openapi/inflect"

	entschema "github.com/huynhanx03/judgify/internal/ent/schema"
)

const (
	maxMigrationFiles = 1_000
	maxMigrationBytes = 8 << 20
	maxDiagnostics    = 100
	maxSQLPreview     = 500
)

type FindingKind string

const (
	FindingDropTable      FindingKind = "drop_table"
	FindingDropColumn     FindingKind = "drop_column"
	FindingDropIndex      FindingKind = "drop_index"
	FindingDropConstraint FindingKind = "drop_constraint"
	FindingDropFunction   FindingKind = "drop_function"
	FindingDropTrigger    FindingKind = "drop_trigger"
	FindingUnknownSQL     FindingKind = "unknown_sql"
)

type MigrationFile struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}

type IdentifierMismatch struct {
	Table         string `json:"table"`
	MigrationType string `json:"migration_type"`
	EntType       string `json:"ent_type"`
}

type Finding struct {
	File      string      `json:"file"`
	Statement int         `json:"statement"`
	Kind      FindingKind `json:"kind"`
	Object    string      `json:"object,omitempty"`
	SQL       string      `json:"sql"`
	Allowed   bool        `json:"allowed"`
	Reason    string      `json:"reason,omitempty"`
}

type Report struct {
	MigrationFiles       []MigrationFile      `json:"migration_files"`
	DeclaredTables       []string             `json:"declared_tables"`
	EntTables            []string             `json:"ent_tables"`
	IdentifierMismatches []IdentifierMismatch `json:"identifier_mismatches"`
	MissingMigrations    []string             `json:"missing_migrations"`
	DestructiveSQL       []Finding            `json:"destructive_sql"`
	Diagnostics          []Finding            `json:"diagnostics"`
	Fingerprint          string               `json:"fingerprint"`
}

type ParseError struct {
	File      string
	Statement int
	Offset    int
	Err       error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf(
		"parse migration %s statement %d at byte %d: %v",
		e.File,
		e.Statement,
		e.Offset,
		e.Err,
	)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

type ValidationError struct {
	IdentifierMismatches int
	MissingMigrations    int
	DestructiveSQL       int
	Diagnostics          int
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf(
		"migration inventory rejected: %d identifier mismatches, %d missing migrations, %d unannotated destructive statements, %d unknown SQL diagnostics",
		e.IdentifierMismatches,
		e.MissingMigrations,
		e.DestructiveSQL,
		e.Diagnostics,
	)
}

func (r Report) Validate() error {
	unannotated := 0
	for _, finding := range r.DestructiveSQL {
		if !finding.Allowed {
			unannotated++
		}
	}

	if len(r.IdentifierMismatches) == 0 && len(r.MissingMigrations) == 0 &&
		unannotated == 0 && len(r.Diagnostics) == 0 {
		return nil
	}
	return &ValidationError{
		IdentifierMismatches: len(r.IdentifierMismatches),
		MissingMigrations:    len(r.MissingMigrations),
		DestructiveSQL:       unannotated,
		Diagnostics:          len(r.Diagnostics),
	}
}

// CurrentSchemas returns every Ent schema owned by the current Judgify API.
// Keeping this list explicit makes a schema addition produce a reviewable diff.
func CurrentSchemas() []ent.Interface {
	return []ent.Interface{
		entschema.ActivatedJudgeCapability{},
		entschema.AttributeDefinition{},
		entschema.AttributeDefinitionRevision{},
		entschema.AuditLog{},
		entschema.AuthRefreshReceipt{},
		entschema.AuthRefreshToken{},
		entschema.AuthSession{},
		entschema.AuthSessionCommandReceipt{},
		entschema.AuthSessionIssuanceReceipt{},
		entschema.AuthorizationAction{},
		entschema.AuthorizationResource{},
		entschema.AuthorizationState{},
		entschema.CasbinRule{},
		entschema.CheckerProfile{},
		entschema.ContentRenderArtifact{},
		entschema.ContentRenderSource{},
		entschema.Contest{},
		entschema.ContestAnnouncement{},
		entschema.ContestClarification{},
		entschema.ContestContentCommand{},
		entschema.ContestContentRevision{},
		entschema.ContestDraftProblem{},
		entschema.ContestProblem{},
		entschema.ContestProblemResult{},
		entschema.ContestRatingResult{},
		entschema.ContestRatingRun{},
		entschema.ContestRegistration{},
		entschema.ContestRegistrationTransition{},
		entschema.ContestScoringAttempt{},
		entschema.ContestStanding{},
		entschema.ContestTransition{},
		entschema.ContestUserRating{},
		entschema.Credential{},
		entschema.CultivationCatalogRevision{},
		entschema.CultivationRankingProjection{},
		entschema.CultivationRankingPrivacyChange{},
		entschema.CultivationRankingState{},
		entschema.Difficulty{},
		entschema.DifficultyRevision{},
		entschema.Element{},
		entschema.ElementRevision{},
		entschema.FederatedIdentity{},
		entschema.JudgeAttempt{},
		entschema.JudgeJob{},
		entschema.JudgeSpec{},
		entschema.JudgeTestResult{},
		entschema.JudgeWorkerCapability{},
		entschema.JudgeWorkerInstance{},
		entschema.Level{},
		entschema.Material{},
		entschema.MaterialCategory{},
		entschema.MaterialRevision{},
		entschema.MaterialRevisionTag{},
		entschema.MaterialSlug{},
		entschema.MaterialViewDaily{},
		entschema.Notification{},
		entschema.NotificationCampaign{},
		entschema.NotificationCampaignConfirmation{},
		entschema.NotificationCampaignExplicitUser{},
		entschema.NotificationCampaignRecipient{},
		entschema.NotificationCampaignRevision{},
		entschema.NotificationInboxState{},
		entschema.NotificationPreference{},
		entschema.OnboardingTraitOffer{},
		entschema.OnboardingTraitOfferTalent{},
		entschema.OAuthCompletionTicket{},
		entschema.OAuthFlow{},
		entschema.Operation{},
		entschema.OperationBatch{},
		entschema.OperationControlReceipt{},
		entschema.OutboxEvent{},
		entschema.PasswordResetToken{},
		entschema.Problem{},
		entschema.ProblemRevision{},
		entschema.ProfileSchemaState{},
		entschema.ProgressionLedgerEntry{},
		entschema.Rank{},
		entschema.Rarity{},
		entschema.RatingHistory{},
		entschema.RatingProjectionGeneration{},
		entschema.RatingRunChain{},
		entschema.RatingRunChainEntry{},
		entschema.RatingSequenceSlot{},
		entschema.RatingState{},
		entschema.RealtimeCursorState{},
		entschema.RealtimeEvent{},
		entschema.RecoveryContactCommandReceipt{},
		entschema.RecoveryContactVerificationToken{},
		entschema.RewardRuleRevision{},
		entschema.Role{},
		entschema.RuntimeProfile{},
		entschema.SecurityEmailDelivery{},
		entschema.SolveRewardCursor{},
		entschema.SolveRewardProcessedTransition{},
		entschema.Submission{},
		entschema.Judgement{},
		entschema.Tag{},
		entschema.TagRevision{},
		entschema.TagRevisionElement{},
		entschema.TestAsset{},
		entschema.TestCase{},
		entschema.TestcaseGroup{},
		entschema.TestcaseRevision{},
		entschema.TestsetRevision{},
		entschema.Trait{},
		entschema.TraitEffectRevision{},
		entschema.User{},
		entschema.UserAttributeValue{},
		entschema.UserDifficultyStats{},
		entschema.UserElementExp{},
		entschema.UserRecoveryContact{},
		entschema.UserProblemSolve{},
		entschema.UserProblemSolveTransition{},
		entschema.UserSolvedProblem{},
		entschema.UserRewardProfileRevision{},
		entschema.UserRewardProfileTrait{},
		entschema.UserStats{},
		entschema.UserTagStats{},
		entschema.UserTrait{},
	}
}

type tableState struct {
	identifierType string
}

type statement struct {
	text string
}

var (
	sqlIdentifier = `(?:"(?:[^"]|"")+"|[a-zA-Z_][a-zA-Z0-9_$]*)`
	qualifiedName = sqlIdentifier + `(?:\s*\.\s*` + sqlIdentifier + `)?`

	createTablePattern    = regexp.MustCompile(`(?is)^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(` + qualifiedName + `)\s*`)
	createFunctionPattern = regexp.MustCompile(`(?is)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?FUNCTION\s+(` + qualifiedName + `)\s*\(`)
	createTriggerPattern  = regexp.MustCompile(`(?is)^\s*CREATE\s+(?:OR\s+REPLACE\s+)?TRIGGER\s+(` + sqlIdentifier + `)\s+.+\s+ON\s+(` + qualifiedName + `)\s+.+\bEXECUTE\s+(?:FUNCTION|PROCEDURE)\s+(` + qualifiedName + `)\s*\(`)
	dropTablePattern      = regexp.MustCompile(`(?is)^\s*DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?(` + qualifiedName + `)`)
	dropFunctionPattern   = regexp.MustCompile(`(?is)^\s*DROP\s+FUNCTION\s+(?:IF\s+EXISTS\s+)?(` + qualifiedName + `)\s*\(`)
	dropTriggerPattern    = regexp.MustCompile(`(?is)^\s*DROP\s+TRIGGER\s+(?:IF\s+EXISTS\s+)?(` + sqlIdentifier + `)\s+ON\s+(` + qualifiedName + `)`)
	alterTablePattern     = regexp.MustCompile(`(?is)^\s*ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(` + qualifiedName + `)`)
	dropIndexPattern      = regexp.MustCompile(`(?is)^\s*DROP\s+INDEX\s+(?:IF\s+EXISTS\s+)?(` + qualifiedName + `)`)
	idColumnPattern       = regexp.MustCompile(`(?is)(?:^|,)\s*(?:"id"|id)\s+([a-zA-Z][a-zA-Z0-9_]*)`)
	dropColumnPattern     = regexp.MustCompile(`(?is)\bDROP\s+COLUMN\s+(?:IF\s+EXISTS\s+)?(` + sqlIdentifier + `)`)
	dropConstraintPattern = regexp.MustCompile(
		`(?is)\bDROP\s+CONSTRAINT\s+(?:IF\s+EXISTS\s+)?(` + sqlIdentifier + `)`,
	)
	allowDestructivePattern = regexp.MustCompile(`(?mi)--\s*inventory:allow-destructive\s+([^\r\n]+)`)
)

func Inspect(ctx context.Context, migrationFS fs.FS, schemas []ent.Interface) (Report, error) {
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	if migrationFS == nil {
		return Report{}, errors.New("migration inventory: nil filesystem")
	}

	entries, err := fs.ReadDir(migrationFS, ".")
	if err != nil {
		return Report{}, fmt.Errorf("read migration directory: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(path.Ext(entry.Name())) != ".sql" {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	if len(names) > maxMigrationFiles {
		return Report{}, fmt.Errorf("migration inventory: %d SQL files exceeds limit %d", len(names), maxMigrationFiles)
	}

	report := Report{
		MigrationFiles:       make([]MigrationFile, 0, len(names)),
		DeclaredTables:       make([]string, 0),
		EntTables:            make([]string, 0, len(schemas)),
		IdentifierMismatches: make([]IdentifierMismatch, 0),
		MissingMigrations:    make([]string, 0),
		DestructiveSQL:       make([]Finding, 0),
		Diagnostics:          make([]Finding, 0),
	}
	tables := make(map[string]tableState)

	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return Report{}, err
		}

		data, err := fs.ReadFile(migrationFS, name)
		if err != nil {
			return Report{}, fmt.Errorf("read migration %s: %w", name, err)
		}
		if len(data) > maxMigrationBytes {
			return Report{}, fmt.Errorf(
				"migration inventory: %s is %d bytes and exceeds limit %d",
				name,
				len(data),
				maxMigrationBytes,
			)
		}

		sum := sha256.Sum256(data)
		report.MigrationFiles = append(report.MigrationFiles, MigrationFile{
			Name:   name,
			SHA256: hex.EncodeToString(sum[:]),
			Bytes:  len(data),
		})

		statements, err := splitStatements(name, string(data))
		if err != nil {
			return Report{}, err
		}
		for index, stmt := range statements {
			if err := ctx.Err(); err != nil {
				return Report{}, err
			}
			if err := analyzeStatement(name, index+1, stmt.text, tables, &report); err != nil {
				return Report{}, err
			}
		}
	}

	entIdentifierTypes := make(map[string]string, len(schemas))
	for _, schema := range schemas {
		if err := ctx.Err(); err != nil {
			return Report{}, err
		}
		table, identifierType, err := describeEntSchema(schema)
		if err != nil {
			return Report{}, err
		}
		if _, exists := entIdentifierTypes[table]; exists {
			return Report{}, fmt.Errorf("migration inventory: duplicate Ent table %q", table)
		}
		entIdentifierTypes[table] = identifierType
		report.EntTables = append(report.EntTables, table)
	}

	for table := range tables {
		report.DeclaredTables = append(report.DeclaredTables, table)
	}
	sort.Strings(report.DeclaredTables)
	sort.Strings(report.EntTables)

	for _, table := range report.EntTables {
		entType := entIdentifierTypes[table]
		migration, exists := tables[table]
		if !exists {
			report.MissingMigrations = append(report.MissingMigrations, table)
			continue
		}
		if migration.identifierType != "" && !identifierTypesEqual(migration.identifierType, entType) {
			report.IdentifierMismatches = append(report.IdentifierMismatches, IdentifierMismatch{
				Table:         table,
				MigrationType: migration.identifierType,
				EntType:       entType,
			})
		}
	}
	sort.Slice(report.IdentifierMismatches, func(i, j int) bool {
		return report.IdentifierMismatches[i].Table < report.IdentifierMismatches[j].Table
	})
	sort.Strings(report.MissingMigrations)
	report.Fingerprint = fingerprint(report)

	return report, nil
}

func analyzeStatement(
	file string,
	statementNumber int,
	raw string,
	tables map[string]tableState,
	report *Report,
) error {
	cleaned := strings.TrimSpace(stripComments(raw))
	if cleaned == "" {
		return nil
	}

	if match := createTablePattern.FindStringSubmatch(cleaned); len(match) > 0 {
		table := normalizeQualifiedName(match[1])
		open := strings.Index(cleaned[createTablePattern.FindStringIndex(cleaned)[1]:], "(")
		if open < 0 {
			return &ParseError{
				File:      file,
				Statement: statementNumber,
				Offset:    0,
				Err:       errors.New("CREATE TABLE is missing opening parenthesis"),
			}
		}
		open += createTablePattern.FindStringIndex(cleaned)[1]
		closeIndex, ok := matchingParen(cleaned, open)
		if !ok {
			return &ParseError{
				File:      file,
				Statement: statementNumber,
				Offset:    open,
				Err:       errors.New("CREATE TABLE has unclosed column list"),
			}
		}

		body := cleaned[open+1 : closeIndex]
		identifierType := ""
		if idMatch := idColumnPattern.FindStringSubmatch(body); len(idMatch) > 0 {
			identifierType = normalizeSQLType(idMatch[1])
		}
		tables[table] = tableState{identifierType: identifierType}
		return nil
	}

	if match := createFunctionPattern.FindStringSubmatch(cleaned); len(match) > 0 {
		return nil
	}

	if match := createTriggerPattern.FindStringSubmatch(cleaned); len(match) > 0 {
		return nil
	}

	if match := dropTablePattern.FindStringSubmatch(cleaned); len(match) > 0 {
		table := normalizeQualifiedName(match[1])
		delete(tables, table)
		report.DestructiveSQL = append(report.DestructiveSQL, destructiveFinding(
			file,
			statementNumber,
			FindingDropTable,
			table,
			raw,
		))
		return nil
	}

	if match := dropFunctionPattern.FindStringSubmatch(cleaned); len(match) > 0 {
		report.DestructiveSQL = append(report.DestructiveSQL, destructiveFinding(
			file,
			statementNumber,
			FindingDropFunction,
			normalizeQualifiedName(match[1]),
			raw,
		))
		return nil
	}

	if match := dropTriggerPattern.FindStringSubmatch(cleaned); len(match) > 0 {
		report.DestructiveSQL = append(report.DestructiveSQL, destructiveFinding(
			file,
			statementNumber,
			FindingDropTrigger,
			unquoteIdentifier(match[1])+"@"+normalizeQualifiedName(match[2]),
			raw,
		))
		return nil
	}

	if match := alterTablePattern.FindStringSubmatch(cleaned); len(match) > 0 {
		table := normalizeQualifiedName(match[1])
		for _, column := range dropColumnPattern.FindAllStringSubmatch(cleaned, -1) {
			name := unquoteIdentifier(column[1])
			if name == "id" {
				state := tables[table]
				state.identifierType = ""
				tables[table] = state
			}
			report.DestructiveSQL = append(report.DestructiveSQL, destructiveFinding(
				file,
				statementNumber,
				FindingDropColumn,
				table+"."+name,
				raw,
			))
		}
		for _, constraint := range dropConstraintPattern.FindAllStringSubmatch(cleaned, -1) {
			report.DestructiveSQL = append(report.DestructiveSQL, destructiveFinding(
				file,
				statementNumber,
				FindingDropConstraint,
				table+"."+unquoteIdentifier(constraint[1]),
				raw,
			))
		}
		return nil
	}

	if match := dropIndexPattern.FindStringSubmatch(cleaned); len(match) > 0 {
		report.DestructiveSQL = append(report.DestructiveSQL, destructiveFinding(
			file,
			statementNumber,
			FindingDropIndex,
			normalizeQualifiedName(match[1]),
			raw,
		))
		return nil
	}

	upper := strings.ToUpper(strings.TrimSpace(cleaned))
	recognizedPrefixes := []string{
		"CREATE INDEX ",
		"CREATE UNIQUE INDEX ",
		"INSERT INTO ",
		"UPDATE ",
		"DELETE FROM ",
		"DO ",
		"SELECT ",
		"SET ",
	}
	for _, prefix := range recognizedPrefixes {
		if strings.HasPrefix(upper, prefix) {
			return nil
		}
	}

	if len(report.Diagnostics) < maxDiagnostics {
		report.Diagnostics = append(report.Diagnostics, Finding{
			File:      file,
			Statement: statementNumber,
			Kind:      FindingUnknownSQL,
			SQL:       previewSQL(raw),
		})
	}
	return nil
}

func destructiveFinding(
	file string,
	statementNumber int,
	kind FindingKind,
	object string,
	raw string,
) Finding {
	finding := Finding{
		File:      file,
		Statement: statementNumber,
		Kind:      kind,
		Object:    object,
		SQL:       previewSQL(raw),
	}
	if match := allowDestructivePattern.FindStringSubmatch(raw); len(match) > 0 {
		finding.Allowed = true
		finding.Reason = strings.TrimSpace(match[1])
	}
	return finding
}

func describeEntSchema(schema ent.Interface) (string, string, error) {
	if schema == nil {
		return "", "", errors.New("migration inventory: nil Ent schema")
	}

	valueType := reflect.TypeOf(schema)
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	if valueType.Name() == "" {
		return "", "", fmt.Errorf("migration inventory: unnamed Ent schema %T", schema)
	}

	var table string
	for _, annotation := range schema.Annotations() {
		switch value := annotation.(type) {
		case entsql.Annotation:
			if value.Table != "" {
				table = value.Table
			}
		case *entsql.Annotation:
			if value != nil && value.Table != "" {
				table = value.Table
			}
		}
	}
	if table == "" {
		table = pluralize(toSnake(valueType.Name()))
	}

	fields := append([]ent.Field(nil), schema.Fields()...)
	for _, mixin := range schema.Mixin() {
		fields = append(fields, mixin.Fields()...)
	}
	for _, schemaField := range fields {
		descriptor := schemaField.Descriptor()
		if descriptor.Err != nil {
			return "", "", fmt.Errorf(
				"migration inventory: schema %s field %s: %w",
				valueType.Name(),
				descriptor.Name,
				descriptor.Err,
			)
		}
		if descriptor.Name != "id" {
			continue
		}
		if descriptor.Info == nil {
			return "", "", fmt.Errorf(
				"migration inventory: schema %s id has no type information",
				valueType.Name(),
			)
		}
		return table, entIdentifierType(descriptor.Info.Type), nil
	}

	// Ent's implicit ID is an integer when no explicit field or mixin overrides it.
	return table, "bigint", nil
}

func entIdentifierType(fieldType entfield.Type) string {
	switch fieldType {
	case entfield.TypeUUID:
		return "uuid"
	case entfield.TypeInt, entfield.TypeInt8, entfield.TypeInt16,
		entfield.TypeInt32, entfield.TypeInt64, entfield.TypeUint,
		entfield.TypeUint8, entfield.TypeUint16, entfield.TypeUint32,
		entfield.TypeUint64:
		return "bigint"
	case entfield.TypeString:
		return "varchar"
	default:
		return strings.ToLower(fieldType.String())
	}
}

func identifierTypesEqual(migrationType, entType string) bool {
	return normalizeSQLType(migrationType) == normalizeSQLType(entType)
}

func normalizeSQLType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "int", "int2", "int4", "int8", "integer", "smallint", "bigserial", "serial", "serial8":
		return "bigint"
	case "character", "varchar", "text":
		return "varchar"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizeQualifiedName(value string) string {
	parts := strings.Split(value, ".")
	return unquoteIdentifier(strings.TrimSpace(parts[len(parts)-1]))
}

func unquoteIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
		value = strings.ReplaceAll(value, `""`, `"`)
	}
	return strings.ToLower(value)
}

func toSnake(value string) string {
	var builder strings.Builder
	runes := []rune(value)
	for index, current := range runes {
		if unicode.IsUpper(current) {
			if index > 0 && (unicode.IsLower(runes[index-1]) ||
				(index+1 < len(runes) && unicode.IsLower(runes[index+1]))) {
				builder.WriteByte('_')
			}
			builder.WriteRune(unicode.ToLower(current))
			continue
		}
		builder.WriteRune(current)
	}
	return builder.String()
}

func pluralize(value string) string {
	// Match Ent's generator instead of maintaining a partial English
	// inflector. A partial rule set silently turned OperationBatch into
	// operation_batchs and made the inventory report disagree with both the
	// generated client and Atlas baseline.
	if strings.HasSuffix(value, "s") {
		return value
	}
	plural := inflect.NewDefaultRuleset().Pluralize(value)
	if plural == value {
		return value + "s"
	}
	return plural
}

func previewSQL(value string) string {
	preview := strings.Join(strings.Fields(value), " ")
	if len(preview) <= maxSQLPreview {
		return preview
	}
	return preview[:maxSQLPreview] + "…"
}

func fingerprint(report Report) string {
	hash := sha256.New()
	for _, file := range report.MigrationFiles {
		_, _ = fmt.Fprintf(hash, "file:%s:%s:%d\n", file.Name, file.SHA256, file.Bytes)
	}
	for _, table := range report.DeclaredTables {
		_, _ = fmt.Fprintf(hash, "migration-table:%s\n", table)
	}
	for _, table := range report.EntTables {
		_, _ = fmt.Fprintf(hash, "ent-table:%s\n", table)
	}
	for _, mismatch := range report.IdentifierMismatches {
		_, _ = fmt.Fprintf(
			hash,
			"mismatch:%s:%s:%s\n",
			mismatch.Table,
			mismatch.MigrationType,
			mismatch.EntType,
		)
	}
	for _, table := range report.MissingMigrations {
		_, _ = fmt.Fprintf(hash, "missing:%s\n", table)
	}
	for _, finding := range report.DestructiveSQL {
		_, _ = fmt.Fprintf(
			hash,
			"destructive:%s:%d:%s:%s:%t:%s\n",
			finding.File,
			finding.Statement,
			finding.Kind,
			finding.Object,
			finding.Allowed,
			finding.Reason,
		)
	}
	for _, diagnostic := range report.Diagnostics {
		_, _ = fmt.Fprintf(
			hash,
			"diagnostic:%s:%d:%s:%s\n",
			diagnostic.File,
			diagnostic.Statement,
			diagnostic.Kind,
			diagnostic.SQL,
		)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func splitStatements(file, sql string) ([]statement, error) {
	statements := make([]statement, 0)
	start := 0
	singleQuoted := false
	doubleQuoted := false
	lineComment := false
	blockComment := false
	dollarTag := ""

	for index := 0; index < len(sql); index++ {
		current := sql[index]

		if lineComment {
			if current == '\n' {
				lineComment = false
			}
			continue
		}
		if blockComment {
			if current == '*' && index+1 < len(sql) && sql[index+1] == '/' {
				blockComment = false
				index++
			}
			continue
		}
		if dollarTag != "" {
			if strings.HasPrefix(sql[index:], dollarTag) {
				index += len(dollarTag) - 1
				dollarTag = ""
			}
			continue
		}
		if singleQuoted {
			if current == '\'' {
				if index+1 < len(sql) && sql[index+1] == '\'' {
					index++
					continue
				}
				singleQuoted = false
			}
			continue
		}
		if doubleQuoted {
			if current == '"' {
				if index+1 < len(sql) && sql[index+1] == '"' {
					index++
					continue
				}
				doubleQuoted = false
			}
			continue
		}

		switch {
		case current == '-' && index+1 < len(sql) && sql[index+1] == '-':
			lineComment = true
			index++
		case current == '/' && index+1 < len(sql) && sql[index+1] == '*':
			blockComment = true
			index++
		case current == '\'':
			singleQuoted = true
		case current == '"':
			doubleQuoted = true
		case current == '$':
			if tag := readDollarTag(sql[index:]); tag != "" {
				dollarTag = tag
				index += len(tag) - 1
			}
		case current == ';':
			if text := strings.TrimSpace(sql[start:index]); text != "" {
				statements = append(statements, statement{text: text})
			}
			start = index + 1
		}
	}

	switch {
	case singleQuoted:
		return nil, &ParseError{File: file, Statement: len(statements) + 1, Offset: len(sql), Err: errors.New("unclosed single-quoted string")}
	case doubleQuoted:
		return nil, &ParseError{File: file, Statement: len(statements) + 1, Offset: len(sql), Err: errors.New("unclosed quoted identifier")}
	case blockComment:
		return nil, &ParseError{File: file, Statement: len(statements) + 1, Offset: len(sql), Err: errors.New("unclosed block comment")}
	case dollarTag != "":
		return nil, &ParseError{File: file, Statement: len(statements) + 1, Offset: len(sql), Err: fmt.Errorf("unclosed dollar quote %s", dollarTag)}
	}

	if text := strings.TrimSpace(sql[start:]); text != "" {
		statements = append(statements, statement{text: text})
	}
	return statements, nil
}

func readDollarTag(value string) string {
	if value == "" || value[0] != '$' {
		return ""
	}
	for index := 1; index < len(value); index++ {
		switch {
		case value[index] == '$':
			return value[:index+1]
		case value[index] == '_' ||
			value[index] >= 'a' && value[index] <= 'z' ||
			value[index] >= 'A' && value[index] <= 'Z' ||
			value[index] >= '0' && value[index] <= '9':
			continue
		default:
			return ""
		}
	}
	return ""
}

func matchingParen(value string, open int) (int, bool) {
	depth := 0
	singleQuoted := false
	doubleQuoted := false
	for index := open; index < len(value); index++ {
		current := value[index]
		if singleQuoted {
			if current == '\'' {
				if index+1 < len(value) && value[index+1] == '\'' {
					index++
					continue
				}
				singleQuoted = false
			}
			continue
		}
		if doubleQuoted {
			if current == '"' {
				if index+1 < len(value) && value[index+1] == '"' {
					index++
					continue
				}
				doubleQuoted = false
			}
			continue
		}
		switch current {
		case '\'':
			singleQuoted = true
		case '"':
			doubleQuoted = true
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return index, true
			}
		}
	}
	return 0, false
}

func stripComments(value string) string {
	var builder strings.Builder
	singleQuoted := false
	doubleQuoted := false
	lineComment := false
	blockComment := false
	dollarTag := ""

	for index := 0; index < len(value); index++ {
		current := value[index]
		if lineComment {
			if current == '\n' {
				lineComment = false
				builder.WriteByte('\n')
			}
			continue
		}
		if blockComment {
			if current == '*' && index+1 < len(value) && value[index+1] == '/' {
				blockComment = false
				index++
			}
			continue
		}
		if dollarTag != "" {
			builder.WriteByte(current)
			if strings.HasPrefix(value[index:], dollarTag) {
				if len(dollarTag) > 1 {
					builder.WriteString(value[index+1 : index+len(dollarTag)])
				}
				index += len(dollarTag) - 1
				dollarTag = ""
			}
			continue
		}
		if singleQuoted {
			builder.WriteByte(current)
			if current == '\'' {
				if index+1 < len(value) && value[index+1] == '\'' {
					builder.WriteByte(value[index+1])
					index++
					continue
				}
				singleQuoted = false
			}
			continue
		}
		if doubleQuoted {
			builder.WriteByte(current)
			if current == '"' {
				if index+1 < len(value) && value[index+1] == '"' {
					builder.WriteByte(value[index+1])
					index++
					continue
				}
				doubleQuoted = false
			}
			continue
		}

		switch {
		case current == '-' && index+1 < len(value) && value[index+1] == '-':
			lineComment = true
			index++
		case current == '/' && index+1 < len(value) && value[index+1] == '*':
			blockComment = true
			index++
		case current == '\'':
			singleQuoted = true
			builder.WriteByte(current)
		case current == '"':
			doubleQuoted = true
			builder.WriteByte(current)
		case current == '$':
			if tag := readDollarTag(value[index:]); tag != "" {
				dollarTag = tag
				builder.WriteString(tag)
				index += len(tag) - 1
			} else {
				builder.WriteByte(current)
			}
		default:
			builder.WriteByte(current)
		}
	}
	return builder.String()
}
