package iptables

type IPTablesRuleSet struct {
	table  string
	chains []string
	rules  []IPTablesRule
}

type IPTablesRule struct {
	chain string
	rule  []string
}

func NewRuleSets() []IPTablesRuleSet {
	return []IPTablesRuleSet{}
}
