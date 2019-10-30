package adblock

import (
	"fmt"
	bluele "github.com/bluele/adblock"
	"github.com/foae/gorgonzola/internal"
	pmezard "github.com/pmezard/adblock/adblock"
	"net/url"
	"os"
	"strings"
	"time"
)

type Servicer interface {
	ShouldBlock(someURL string) (bool, error)
}

type Service struct {
	adblockRules []*bluele.Rules
	adbRules     []*pmezard.RuleMatcher
	log          internal.Logger
}

func NewService(logger internal.Logger) *Service {
	return &Service{
		adblockRules: make([]*bluele.Rules, 0),
		adbRules:     make([]*pmezard.RuleMatcher, 0),
		log:          logger,
	}
}

func (svc *Service) LoadAdBlockPlusProviders(filePath string) error {
	head, err := PeekFile(filePath, 64)
	if err != nil {
		return fmt.Errorf("could not read file (%v): %v", filePath, err)
	}

	if IsFileAdBlockPlusFormat(head) == false {
		return fmt.Errorf("file contents of (%v) are not of type AdBlock Plus: (%s)", filePath, head)
	}

	/*
		Load <bluele> provider
	*/
	svc.log.Debug("Loading <bluele>... ", filePath)
	adblockRule, err := bluele.NewRulesFromFile(filePath, &bluele.RulesOption{
		Supports:              nil,
		CheckUnsupportedRules: true,
	})
	switch {
	case err != nil:
		svc.log.Debugf("could not init <bluele> rules: %v", err)
	default:
		svc.adblockRules = append(svc.adblockRules, adblockRule)
	}

	/*
		LOAD <pmezard> provider
	*/
	matcher := pmezard.NewMatcher()
	fp, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("could not open file: %v", err)
	}

	rules, err := pmezard.ParseRules(fp)
	switch {
	case err != nil:
		svc.log.Errorf("could not open file (%v): %v", fp.Name(), err)
	default:
		for idx, rule := range rules {
			if err = matcher.AddRule(rule, idx); err != nil {
				svc.log.Debugf("could not add rule #%v (%v): %v", idx, rule.Raw, err)
				continue
			}
		}

		_ = fp.Close()
		svc.adbRules = append(svc.adbRules, matcher)
		svc.log.Debugf("Loaded (%v) <pmezard> rules from file (%v).", len(rules), filePath)
	}

	return nil
}

func (svc *Service) ShouldBlock(someURL string) (bool, error) {
	someURL = strings.TrimSuffix(someURL, ".")
	uurl, err := url.Parse(someURL)
	if err != nil {
		return false, fmt.Errorf("(%v) is not a valid URL: %v", someURL, err)
	}

	var shouldBlock bool

	for idx, rule := range svc.adblockRules {
		ts := time.Now()
		found := rule.ShouldBlock(someURL, map[string]interface{}{
			"domain": uurl.Hostname(),
		})
		if found {
			shouldBlock = true
		}

		svc.log.Debugf("Searched for (%v) in (<bluele> #%v: %v): %v", uurl, idx, time.Since(ts), found)
	}

	for idx, rule := range svc.adbRules {
		ts := time.Now()
		found, _, err := rule.Match(&pmezard.Request{
			URL:       someURL,
			Domain:    uurl.Hostname(),
			Timeout:   time.Millisecond * 100,
			CheckFreq: 0,
		})
		if err != nil {
			svc.log.Debugf("pmezard: %v", err)
		}
		if found {
			shouldBlock = true
		}

		svc.log.Debugf("Searched for (%v) in (<pmezard> #%v: %v): %v", someURL, idx, time.Since(ts), found)
	}

	return shouldBlock, nil
}
