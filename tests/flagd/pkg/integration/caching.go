package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cucumber/godog"
	"github.com/open-feature/flagd/core/pkg/eval"
	"github.com/open-feature/flagd/core/pkg/model"
	flagd "github.com/open-feature/go-sdk-contrib/providers/flagd/pkg"
	"github.com/open-feature/go-sdk/pkg/openfeature"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var (
	testingFlags                                                                         eval.Flags
	flagConfigurationPath, flagConfigurationTargetFilePath, flagMutatedConfigurationPath string
)

// InitializeCachingScenario initializes the caching test scenario
//
// All the path parameters must be in the same dir.
// flagConfigTargetFilePath - must be the path to the underlying flag configuration file
// flagConfigPath - must be the path to a symlink to flagConfigTargetFilePath
// flagMutatedConfigPath - must be the path to a non-existing flag configuration file
func InitializeCachingScenario(flagConfigTargetFilePath, flagConfigPath, flagMutatedConfigPath string, pOptions ...flagd.ProviderOption) (func(*godog.ScenarioContext), error) {
	providerOptions = pOptions
	flagConfigurationPath = flagConfigPath
	flagConfigurationTargetFilePath = flagConfigTargetFilePath
	flagMutatedConfigurationPath = flagMutatedConfigPath
	var err error
	testingFlags, err = loadFlagConfiguration(flagConfigPath)
	if err != nil {
		return nil, err
	}

	err = writeFlagsToFile(testingFlags, flagMutatedConfigurationPath)
	if err != nil {
		return nil, err
	}

	return initializeCachingScenario, nil
}

func initializeCachingScenario(ctx *godog.ScenarioContext) {
	ctx.Step(`^a boolean flag with key "([^"]*)" is evaluated with details and default value "([^"]*)"$`, aBooleanFlagWithKeyIsEvaluatedWithDetailsAndDefaultValue)
	ctx.Step(`^a float flag with key "([^"]*)" is evaluated with details and default value (\d+)\.(\d+)$`, aFloatFlagWithKeyIsEvaluatedWithDetailsAndDefaultValue)
	ctx.Step(`^a string flag with key "([^"]*)" is evaluated with details and default value "([^"]*)"$`, aStringFlagWithKeyIsEvaluatedWithDetailsAndDefaultValue)
	ctx.Step(`^an integer flag with key "([^"]*)" is evaluated with details and default value (\d+)$`, anIntegerFlagWithKeyIsEvaluatedWithDetailsAndDefaultValue)
	ctx.Step(`^an object flag with key "([^"]*)" is evaluated with details and a null default value$`, anObjectFlagWithKeyIsEvaluatedWithDetailsAndANullDefaultValue)
	ctx.Step(`^a provider is registered with cache enabled$`, aProviderIsRegisteredWithCacheEnabled)
	ctx.Step(`^sleep for (\d+) milliseconds$`, sleepForMilliseconds)
	ctx.Step(`^the flag\'s configuration with key "([^"]*)" is updated to defaultVariant "([^"]*)"$`, theFlagsConfigurationWithKeyIsUpdatedToDefaultVariant)
	ctx.Step(`^the resolved boolean details reason of flag with key "([^"]*)" should be "([^"]*)"$`, theResolvedBooleanDetailsReasonOfFlagWithKeyShouldBe)
	ctx.Step(`^the resolved boolean details reason should be "([^"]*)"$`, theResolvedBooleanDetailsReasonShouldBe)
	ctx.Step(`^the resolved float details reason of flag with key "([^"]*)" should be "([^"]*)"$`, theResolvedFloatDetailsReasonOfFlagWithKeyShouldBe)
	ctx.Step(`^the resolved float details reason should be "([^"]*)"$`, theResolvedFloatDetailsReasonShouldBe)
	ctx.Step(`^the resolved integer details reason of flag with key "([^"]*)" should be "([^"]*)"$`, theResolvedIntegerDetailsReasonOfFlagWithKeyShouldBe)
	ctx.Step(`^the resolved integer details reason should be "([^"]*)"$`, theResolvedIntegerDetailsReasonShouldBe)
	ctx.Step(`^the resolved object details reason of flag with key "([^"]*)" should be "([^"]*)"$`, theResolvedObjectDetailsReasonOfFlagWithKeyShouldBe)
	ctx.Step(`^the resolved object details reason should be "([^"]*)"$`, theResolvedObjectDetailsReasonShouldBe)
	ctx.Step(`^the resolved string details reason of flag with key "([^"]*)" should be "([^"]*)"$`, theResolvedStringDetailsReasonOfFlagWithKeyShouldBe)
	ctx.Step(`^the resolved string details reason should be "([^"]*)"$`, theResolvedStringDetailsReasonShouldBe)

	ctx.Before(resetState)
}

func loadFlagConfiguration(path string) (eval.Flags, error) {
	file, err := os.Open(path)
	if err != nil {
		return eval.Flags{}, err
	}

	var flags eval.Flags
	err = json.NewDecoder(file).Decode(&flags)
	if err != nil {
		return eval.Flags{}, fmt.Errorf("decode %s: %v", path, err)
	}

	return flags, nil
}

func writeFlagsToFile(flags eval.Flags, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}

	err = json.NewEncoder(file).Encode(flags)
	if err != nil {
		return fmt.Errorf("write flag configuration to file: %w", err)
	}

	return nil
}

func theFlagsConfigurationWithKeyIsUpdatedToDefaultVariant(flagKey, defaultVariant string) error {
	flags := copyFlags(testingFlags)
	flagConfig := flags.Flags[flagKey]
	flagConfig.DefaultVariant = defaultVariant
	flags.Flags[flagKey] = flagConfig

	if err := writeFlagsToFile(flags, flagMutatedConfigurationPath); err != nil {
		return fmt.Errorf("writeFlagsToFile %s: %w", flagMutatedConfigurationPath, err)
	}

	cmd := exec.Command("ln", "-sf", filepath.Base(flagMutatedConfigurationPath), flagConfigurationPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("symlink: %w", err)
	}

	// remove the underlying linked to file, forcing the symlink to be reevaluated by flagd.
	// without this, the symlink change isn't noticed by flagd
	if err := os.Remove(flagConfigurationTargetFilePath); err != nil {
		return fmt.Errorf("remove %s: %w", flagConfigurationTargetFilePath, err)
	}

	return nil
}

func theResolvedBooleanDetailsReasonOfFlagWithKeyShouldBe(ctx context.Context, flagkey, reason string) error {
	store, ok := ctx.Value(ctxStorageKey{}).(map[string]openfeature.BooleanEvaluationDetails)
	if !ok {
		return errors.New("no flag resolution result")
	}

	got, ok := store[flagkey]
	if !ok {
		return fmt.Errorf("flag result with key %s not found", flagkey)
	}

	if string(got.Reason) != reason {
		return fmt.Errorf("expected reason to be %s, got %s", reason, got.Reason)
	}

	return nil
}

func theResolvedStringDetailsReasonOfFlagWithKeyShouldBe(ctx context.Context, flagkey, reason string) error {
	store, ok := ctx.Value(ctxStorageKey{}).(map[string]openfeature.StringEvaluationDetails)
	if !ok {
		return errors.New("no flag resolution result")
	}

	got, ok := store[flagkey]
	if !ok {
		return fmt.Errorf("flag result with key %s not found", flagkey)
	}

	if string(got.Reason) != reason {
		return fmt.Errorf("expected reason to be %s, got %s", reason, got.Reason)
	}

	return nil
}

func theResolvedIntegerDetailsReasonOfFlagWithKeyShouldBe(ctx context.Context, flagkey, reason string) error {
	store, ok := ctx.Value(ctxStorageKey{}).(map[string]openfeature.IntEvaluationDetails)
	if !ok {
		return errors.New("no flag resolution result")
	}

	got, ok := store[flagkey]
	if !ok {
		return fmt.Errorf("flag result with key %s not found", flagkey)
	}

	if string(got.Reason) != reason {
		return fmt.Errorf("expected reason to be %s, got %s", reason, got.Reason)
	}

	return nil
}

func theResolvedFloatDetailsReasonOfFlagWithKeyShouldBe(ctx context.Context, flagkey, reason string) error {
	store, ok := ctx.Value(ctxStorageKey{}).(map[string]openfeature.FloatEvaluationDetails)
	if !ok {
		return errors.New("no flag resolution result")
	}

	got, ok := store[flagkey]
	if !ok {
		return fmt.Errorf("flag result with key %s not found", flagkey)
	}

	if string(got.Reason) != reason {
		return fmt.Errorf("expected reason to be %s, got %s", reason, got.Reason)
	}

	return nil
}

func theResolvedObjectDetailsReasonOfFlagWithKeyShouldBe(ctx context.Context, flagkey, reason string) error {
	store, ok := ctx.Value(ctxStorageKey{}).(map[string]openfeature.InterfaceEvaluationDetails)
	if !ok {
		return errors.New("no flag resolution result")
	}

	got, ok := store[flagkey]
	if !ok {
		return fmt.Errorf("flag result with key %s not found", flagkey)
	}

	if string(got.Reason) != reason {
		return fmt.Errorf("expected reason to be %s, got %s", reason, got.Reason)
	}

	return nil
}

func theResolvedBooleanDetailsReasonShouldBe(ctx context.Context, reason string) error {
	got, err := firstBooleanEvaluationDetails(ctx)
	if err != nil {
		return err
	}

	if string(got.Reason) != reason {
		return fmt.Errorf("expected reason to be %s, got %s", reason, got.Reason)
	}

	return nil
}

func theResolvedStringDetailsReasonShouldBe(ctx context.Context, reason string) error {
	got, err := firstStringEvaluationDetails(ctx)
	if err != nil {
		return err
	}

	if string(got.Reason) != reason {
		return fmt.Errorf("expected reason to be %s, got %s", reason, got.Reason)
	}

	return nil
}

func theResolvedIntegerDetailsReasonShouldBe(ctx context.Context, reason string) error {
	got, err := firstIntegerEvaluationDetails(ctx)
	if err != nil {
		return err
	}

	if string(got.Reason) != reason {
		return fmt.Errorf("expected reason to be %s, got %s", reason, got.Reason)
	}

	return nil
}

func theResolvedFloatDetailsReasonShouldBe(ctx context.Context, reason string) error {
	got, err := firstFloatEvaluationDetails(ctx)
	if err != nil {
		return err
	}

	if string(got.Reason) != reason {
		return fmt.Errorf("expected reason to be %s, got %s", reason, got.Reason)
	}

	return nil
}

func theResolvedObjectDetailsReasonShouldBe(ctx context.Context, reason string) error {
	got, err := firstInterfaceEvaluationDetails(ctx)
	if err != nil {
		return err
	}

	if string(got.Reason) != reason {
		return fmt.Errorf("expected reason to be %s, got %s", reason, got.Reason)
	}

	return nil
}

func sleepForMilliseconds(milliseconds int64) error {
	time.Sleep(time.Duration(milliseconds) * time.Millisecond)
	return nil
}

func resetState(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
	if flagConfigurationTargetFilePath == "" {
		// nothing to reset
		return ctx, nil
	}

	if _, err := os.Stat(flagConfigurationTargetFilePath); err == nil {
		// initial flag configuration file already exists, no reset necessary
		return ctx, nil
	}

	if err := writeFlagsToFile(testingFlags, flagConfigurationTargetFilePath); err != nil {
		return ctx, err
	}

	cmd := exec.Command("ln", "-sf", filepath.Base(flagConfigurationTargetFilePath), flagConfigurationPath)
	if err := cmd.Run(); err != nil {
		return ctx, fmt.Errorf("symlink: %w", err)
	}

	if err := os.Remove(flagMutatedConfigurationPath); err != nil {
		return ctx, fmt.Errorf("remove %s: %w", flagMutatedConfigurationPath, err)
	}

	return ctx, nil
}

func aProviderIsRegisteredWithCacheEnabled(ctx context.Context) (context.Context, error) {
	pOptions := []flagd.ProviderOption{flagd.WithPort(8013)}
	pOptions = append(pOptions, providerOptions...)
	provider := flagd.NewProvider(pOptions...)
	openfeature.SetProvider(provider)
	client := openfeature.NewClient("caching tests")

	select {
	case <-provider.IsReady():
	case <-time.After(500 * time.Millisecond):
		return ctx, errors.New("provider not ready after 500 milliseconds")
	}

	return context.WithValue(ctx, ctxClientKey{}, client), nil
}

func copyFlags(flags eval.Flags) eval.Flags {
	f := eval.Flags{Flags: map[string]model.Flag{}}

	for key, flag := range flags.Flags {
		f.Flags[key] = flag
	}

	return f
}
