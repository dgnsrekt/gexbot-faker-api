package config

import (
	"fmt"
	"sort"
	"strings"
)

// InvalidCategory represents an invalid package/category combination
type InvalidCategory struct {
	Package  string
	Category string
}

// ValidationErrors collects all validation errors
type ValidationErrors struct {
	InvalidTickers    []string
	InvalidPackages   []string
	InvalidCategories []InvalidCategory
}

// HasErrors returns true if any validation errors exist
func (e *ValidationErrors) HasErrors() bool {
	return len(e.InvalidTickers) > 0 || len(e.InvalidPackages) > 0 || len(e.InvalidCategories) > 0
}

// Error formats all validation errors into a clear message
func (e *ValidationErrors) Error() string {
	var sb strings.Builder
	sb.WriteString("configuration validation failed:\n")

	if len(e.InvalidTickers) > 0 {
		sb.WriteString("\nInvalid tickers:\n")
		for _, t := range e.InvalidTickers {
			sb.WriteString(fmt.Sprintf("  - %s\n", t))
		}
		sb.WriteString(fmt.Sprintf("\nValid tickers: %s\n", validTickersList()))
	}

	if len(e.InvalidPackages) > 0 {
		sb.WriteString("\nInvalid packages:\n")
		for _, p := range e.InvalidPackages {
			sb.WriteString(fmt.Sprintf("  - %s\n", p))
		}
		sb.WriteString("\nValid packages: state, classic, orderflow\n")
	}

	if len(e.InvalidCategories) > 0 {
		sb.WriteString("\nInvalid package/category combinations:\n")
		for _, ic := range e.InvalidCategories {
			validCats := ValidCategories[Package(ic.Package)]
			sb.WriteString(fmt.Sprintf("  - %s/%s (%s only supports: %s)\n",
				ic.Package, ic.Category, ic.Package, strings.Join(validCats, ", ")))
		}
	}

	return sb.String()
}

// ValidateDownloadConfig validates tickers and package/category combinations
func ValidateDownloadConfig(tickers []string, packages PackagesConfig) error {
	errs := &ValidationErrors{}

	// Validate tickers
	for _, ticker := range tickers {
		if !ValidTickers[ticker] {
			errs.InvalidTickers = append(errs.InvalidTickers, ticker)
		}
	}

	// Validate package/category combinations
	validatePackageCategories(errs, "state", packages.State)
	validatePackageCategories(errs, "classic", packages.Classic)
	validatePackageCategories(errs, "orderflow", packages.Orderflow)

	if errs.HasErrors() {
		return errs
	}
	return nil
}

func validatePackageCategories(errs *ValidationErrors, pkgName string, pkg PackageConfig) {
	if !pkg.Enabled {
		return
	}

	validCats := ValidCategories[Package(pkgName)]
	validCatsMap := make(map[string]bool)
	for _, c := range validCats {
		validCatsMap[c] = true
	}

	for _, category := range pkg.Categories {
		if !validCatsMap[category] {
			errs.InvalidCategories = append(errs.InvalidCategories, InvalidCategory{
				Package:  pkgName,
				Category: category,
			})
		}
	}
}

func validTickersList() string {
	tickers := make([]string, 0, len(ValidTickers))
	for t := range ValidTickers {
		tickers = append(tickers, t)
	}
	sort.Strings(tickers)
	return strings.Join(tickers, ", ")
}
