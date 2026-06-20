package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/google/cel-go/cel"
)

type PolicyCondition struct {
	Abilities     []string `json:"abilities,omitempty"`
	RuntimeGroups []string `json:"runtime_groups,omitempty"`
	ModelPrefixes []string `json:"model_prefixes,omitempty"`
	ModelNames    []string `json:"model_names,omitempty"`
	ChannelIds    []int    `json:"channel_ids,omitempty"`
	IsPlayground  *bool    `json:"is_playground,omitempty"`
}

type EnterprisePolicyCELInput struct {
	User    EnterprisePolicyCELUser
	Org     EnterprisePolicyCELOrg
	Request EnterprisePolicyCELRequest
	Token   EnterprisePolicyCELToken
}

type EnterprisePolicyCELUser struct {
	Id           int
	RuntimeGroup string
	Role         string
}

type EnterprisePolicyCELOrg struct {
	EnterpriseId   int
	OrgUnitId      int
	OrgUnitPathIds []int
	PolicyGroupIds []int
	ProjectId      int
}

type EnterprisePolicyCELRequest struct {
	Model        string
	Ability      string
	IsPlayground bool
	ChannelId    int
}

type EnterprisePolicyCELToken struct {
	Id int
}

type compiledEnterprisePolicyCondition struct {
	expr    string
	program cel.Program
}

var enterprisePolicyConditionCache = struct {
	sync.RWMutex
	items map[string]compiledEnterprisePolicyCondition
}{
	items: map[string]compiledEnterprisePolicyCondition{},
}

func NormalizeEnterpriseQuotaPolicyCondition(policy *model.EnterpriseQuotaPolicy) error {
	mode := strings.TrimSpace(policy.ConditionMode)
	if mode == "" {
		mode = model.PolicyConditionModeStructured
	}
	switch mode {
	case model.PolicyConditionModeStructured:
		condition, err := ParsePolicyCondition(policy.ConditionJson)
		if err != nil {
			return err
		}
		condition.Normalize()
		conditionJson := ""
		if !condition.IsEmpty() {
			encoded, err := common.Marshal(condition)
			if err != nil {
				return err
			}
			conditionJson = string(encoded)
		}
		expr, err := BuildCELExpressionFromCondition(condition)
		if err != nil {
			return err
		}
		if err := ValidatePolicyConditionExpression(expr); err != nil {
			return err
		}
		policy.ConditionMode = mode
		policy.ConditionJson = conditionJson
		policy.ConditionExpr = expr
	case model.PolicyConditionModeCEL:
		expr := strings.TrimSpace(policy.ConditionExpr)
		if expr == "" {
			expr = "true"
		}
		if err := ValidatePolicyConditionExpression(expr); err != nil {
			return err
		}
		policy.ConditionMode = mode
		policy.ConditionExpr = expr
	default:
		return fmt.Errorf("不支持的策略条件模式: %s", mode)
	}
	policy.ConditionHash = hashEnterprisePolicyCondition(policy.ConditionMode, policy.ConditionJson, policy.ConditionExpr)
	return nil
}

func ParsePolicyCondition(raw string) (PolicyCondition, error) {
	var condition PolicyCondition
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return condition, nil
	}
	if err := common.Unmarshal([]byte(raw), &condition); err != nil {
		return condition, fmt.Errorf("策略条件 JSON 无效: %w", err)
	}
	return condition, nil
}

func (condition *PolicyCondition) Normalize() {
	condition.Abilities = normalizeEnterpriseConditionStrings(condition.Abilities)
	condition.RuntimeGroups = normalizeEnterpriseConditionStrings(condition.RuntimeGroups)
	condition.ModelPrefixes = normalizeEnterpriseConditionStrings(condition.ModelPrefixes)
	condition.ModelNames = normalizeEnterpriseConditionStrings(condition.ModelNames)
	condition.ChannelIds = normalizeEnterpriseConditionInts(condition.ChannelIds)
}

func (condition PolicyCondition) IsEmpty() bool {
	return len(condition.Abilities) == 0 &&
		len(condition.RuntimeGroups) == 0 &&
		len(condition.ModelPrefixes) == 0 &&
		len(condition.ModelNames) == 0 &&
		len(condition.ChannelIds) == 0 &&
		condition.IsPlayground == nil
}

func BuildCELExpressionFromCondition(condition PolicyCondition) (string, error) {
	condition.Normalize()
	parts := make([]string, 0, 6)
	if len(condition.Abilities) > 0 {
		parts = append(parts, fmt.Sprintf("request.ability in %s", celStringList(condition.Abilities)))
	}
	if len(condition.RuntimeGroups) > 0 {
		parts = append(parts, fmt.Sprintf("user.runtime_group in %s", celStringList(condition.RuntimeGroups)))
	}
	if len(condition.ModelNames) > 0 {
		parts = append(parts, fmt.Sprintf("request.model in %s", celStringList(condition.ModelNames)))
	}
	if len(condition.ModelPrefixes) > 0 {
		parts = append(parts, fmt.Sprintf("%s.exists(prefix, request.model.startsWith(prefix))", celStringList(condition.ModelPrefixes)))
	}
	if len(condition.ChannelIds) > 0 {
		parts = append(parts, fmt.Sprintf("request.channel_id in %s", celIntList(condition.ChannelIds)))
	}
	if condition.IsPlayground != nil {
		parts = append(parts, fmt.Sprintf("request.is_playground == %t", *condition.IsPlayground))
	}
	if len(parts) == 0 {
		return "true", nil
	}
	return strings.Join(parts, " && "), nil
}

func ValidatePolicyConditionExpression(expr string) error {
	_, err := compileEnterprisePolicyCondition(strings.TrimSpace(expr))
	return err
}

func EvaluatePolicyCondition(policy model.EnterpriseQuotaPolicy, input EnterprisePolicyCELInput) (bool, error) {
	expr := strings.TrimSpace(policy.ConditionExpr)
	if expr == "" {
		expr = "true"
	}
	cacheKey := enterprisePolicyConditionCacheKey(policy)
	enterprisePolicyConditionCache.RLock()
	compiled, ok := enterprisePolicyConditionCache.items[cacheKey]
	enterprisePolicyConditionCache.RUnlock()
	if !ok || compiled.expr != expr {
		program, err := compileEnterprisePolicyCondition(expr)
		if err != nil {
			return false, err
		}
		compiled = compiledEnterprisePolicyCondition{expr: expr, program: program}
		enterprisePolicyConditionCache.Lock()
		enterprisePolicyConditionCache.items[cacheKey] = compiled
		enterprisePolicyConditionCache.Unlock()
	}
	out, _, err := compiled.program.Eval(enterprisePolicyCELActivation(input))
	if err != nil {
		return false, err
	}
	value, ok := out.Value().(bool)
	if !ok {
		return false, errors.New("策略条件表达式返回值不是 bool")
	}
	return value, nil
}

func compileEnterprisePolicyCondition(expr string) (cel.Program, error) {
	if expr == "" {
		expr = "true"
	}
	env, err := enterprisePolicyCELEnv()
	if err != nil {
		return nil, err
	}
	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}
	if ast.OutputType() == nil || !ast.OutputType().IsExactType(cel.BoolType) {
		return nil, errors.New("策略条件表达式必须返回 bool")
	}
	return env.Program(ast)
}

func enterprisePolicyCELEnv() (*cel.Env, error) {
	return cel.NewEnv(
		cel.Variable("user.id", cel.IntType),
		cel.Variable("user.runtime_group", cel.StringType),
		cel.Variable("user.role", cel.StringType),
		cel.Variable("org.enterprise_id", cel.IntType),
		cel.Variable("org.org_unit_id", cel.IntType),
		cel.Variable("org.org_unit_path_ids", cel.ListType(cel.IntType)),
		cel.Variable("org.policy_group_ids", cel.ListType(cel.IntType)),
		cel.Variable("org.project_id", cel.IntType),
		cel.Variable("request.model", cel.StringType),
		cel.Variable("request.ability", cel.StringType),
		cel.Variable("request.is_playground", cel.BoolType),
		cel.Variable("request.channel_id", cel.IntType),
		cel.Variable("token.id", cel.IntType),
	)
}

func enterprisePolicyCELActivation(input EnterprisePolicyCELInput) map[string]any {
	return map[string]any{
		"user.id":               int64(input.User.Id),
		"user.runtime_group":    input.User.RuntimeGroup,
		"user.role":             input.User.Role,
		"org.enterprise_id":     int64(input.Org.EnterpriseId),
		"org.org_unit_id":       int64(input.Org.OrgUnitId),
		"org.org_unit_path_ids": intSliceToInt64Slice(input.Org.OrgUnitPathIds),
		"org.policy_group_ids":  intSliceToInt64Slice(input.Org.PolicyGroupIds),
		"org.project_id":        int64(input.Org.ProjectId),
		"request.model":         input.Request.Model,
		"request.ability":       input.Request.Ability,
		"request.is_playground": input.Request.IsPlayground,
		"request.channel_id":    int64(input.Request.ChannelId),
		"token.id":              int64(input.Token.Id),
	}
}

func enterprisePolicyConditionCacheKey(policy model.EnterpriseQuotaPolicy) string {
	hash := policy.ConditionHash
	if hash == "" {
		hash = hashEnterprisePolicyCondition(policy.ConditionMode, policy.ConditionJson, policy.ConditionExpr)
	}
	return fmt.Sprintf("%d:%s:%d", policy.Id, hash, policy.UpdatedAt)
}

func hashEnterprisePolicyCondition(mode string, conditionJson string, conditionExpr string) string {
	sum := sha256.Sum256([]byte(mode + "\n" + conditionJson + "\n" + conditionExpr))
	return hex.EncodeToString(sum[:])
}

func normalizeEnterpriseConditionStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func normalizeEnterpriseConditionInts(values []int) []int {
	result := make([]int, 0, len(values))
	seen := map[int]struct{}{}
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Ints(result)
	return result
}

func celStringList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, strconv.Quote(value))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func celIntList(values []int) string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, strconv.Itoa(value))
	}
	return "[" + strings.Join(items, ", ") + "]"
}

func intSliceToInt64Slice(values []int) []int64 {
	result := make([]int64, 0, len(values))
	for _, value := range values {
		result = append(result, int64(value))
	}
	return result
}
