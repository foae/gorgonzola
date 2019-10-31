package adblock

import (
	"fmt"
	bluele "github.com/bluele/adblock"
	"github.com/foae/gorgonzola/internal"
	pmezard "github.com/pmezard/adblock/adblock"
	"net/url"
	"time"
)

// Servicer describes the needed functionality
// to interact with this package.
type Servicer interface {
	ShouldBlock(someURL string) (bool, error)
}

// Service describes the intimate (woah)
// structure of a Servicer implementation.
type Service struct {
	adblockRules []*bluele.Rules
	adbRules     *pmezard.RuleMatcher
	logger       internal.Logger
}

// NewService returns an instance of the Service
// based on configurable dependencies.
func NewService(logger internal.Logger) *Service {
	return &Service{
		adblockRules: make([]*bluele.Rules, 0),
		adbRules:     new(pmezard.RuleMatcher),
		logger:       logger,
	}
}

// LoadAdBlockPlusProviders reads, validates, parses and loads into memory
// a given list of text files containing AdBlock Plus rules.
func (svc *Service) LoadAdBlockPlusProviders(files []string) error {
	/*
		Validate incoming files.
	*/
	collectedFiles := make([]string, 0, len(files))
	svc.logger.Debug("Parsing AdBlock Plus providers. This might take up to 2 minutes...")
	for _, f := range files {
		head, err := PeekFile(f, 64)
		if err != nil {
			continue
		}

		if IsFileAdBlockPlusFormat(head) == false {
			continue
		}

		collectedFiles = append(collectedFiles, f)
	}

	/*
		Load <pmezard> provider
	*/
	if err := svc.loadAdBlockPlusProviderPmezard(collectedFiles); err != nil {
		return err
	}

	/*
		Load <bluele> provider
	*/
	for _, cf := range collectedFiles {
		if err := svc.loadAdBlockPlusProviderBluele(cf); err != nil {
			return err
		}
	}

	return nil
}

// <<pmezard>>
func (svc *Service) loadAdBlockPlusProviderPmezard(files []string) error {
	matcher, rulesNo, err := pmezard.NewMatcherFromFiles(files...)
	switch {
	case err != nil:
		return err
	default:
		svc.adbRules = matcher
		svc.logger.Debugf("Loaded <pmezard> with (%v) rules.", rulesNo)
	}

	return nil
}

// <<bluele>>
func (svc *Service) loadAdBlockPlusProviderBluele(filePath string) error {
	adblockRules, err := bluele.NewRulesFromFile(filePath, &bluele.RulesOption{
		Supports:              nil,
		CheckUnsupportedRules: false,
	})
	switch {
	case err != nil:
		return err
	default:
		svc.adblockRules = append(svc.adblockRules, adblockRules)
		svc.logger.Debugf("Loaded <bluele> with (%v) rules.", len(adblockRules.BlackList()))
	}

	return nil
}

// ShouldBlock decides whether a given URL should be blocked or not.
// Minimum validation is done, the caller should take care
// of sending a correctly formatted FQDN.
func (svc *Service) ShouldBlock(someURL string) (bool, error) {
	uurl, err := url.Parse(someURL)
	if err != nil {
		return false, fmt.Errorf("(%v) is not a valid URL: %v", someURL, err)
	}

	var shouldBlock bool

	ts := time.Now()
	found, ruleNo, err := svc.adbRules.Match(&pmezard.Request{
		URL:       uurl.String(),
		Domain:    uurl.Hostname(),
		Timeout:   time.Millisecond * 500,
		CheckFreq: 0,
	})
	switch {
	case err != nil:
		svc.logger.Debugf("pmezard error: %v", err)
	case found:
		shouldBlock = true
		svc.logger.Debugf("Searched for (%v) in (<pmezard> rule #%v: %v): %v", uurl, ruleNo, time.Since(ts), found)

	}

	for idx, rule := range svc.adblockRules {
		ts := time.Now()
		found := rule.ShouldBlock(uurl.String(), map[string]interface{}{
			"domain": uurl.Hostname(),
		})
		if found {
			shouldBlock = true
			svc.logger.Debugf("Searched for (%v) in (<bluele> #%v: %v): %v", uurl, idx, time.Since(ts), found)
		}
	}

	return shouldBlock, nil
}
