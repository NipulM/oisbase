package utils

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// OIQPriceOutput represents the JSON output from oiq price.
type OIQPriceOutput struct {
	MatchDate string        `json:"match_date"`
	PrevPrice MinMaxPrice   `json:"prev_price"`
	Price     MinMaxPrice   `json:"price"`
	PriceDiff MinMaxPrice   `json:"price_diff"`
	Resources []OIQResource `json:"resources"`
}

// MinMaxPrice holds a min/max price range.
type MinMaxPrice struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// OIQResource represents a single resource in the oiq output.
type OIQResource struct {
	Address  string       `json:"address"`
	Change   string       `json:"change"`
	Name     string       `json:"name"`
	Price    MinMaxPrice  `json:"price"`
	Type     string       `json:"type"`
	Products []OIQProduct `json:"products"`
}

// OIQProduct represents a single pricing line item within a resource.
type OIQProduct struct {
	Price      MinMaxPrice     `json:"price"`
	ProductMax OIQProductInfo  `json:"product_max"`
	ProductMin OIQProductInfo  `json:"product_min"`
	Usage      OIQUsage        `json:"usage"`
}

// OIQProductInfo holds service/pricing metadata.
type OIQProductInfo struct {
	CCY            string `json:"ccy"`
	ProductFamily  string `json:"product_family"`
	Service        string `json:"service"`
	PricingMatchSet string `json:"pricing_match_set"`
}

// OIQUsage holds the usage assumptions for a product line.
type OIQUsage struct {
	Description string          `json:"description"`
	MatchQuery  string          `json:"match_query"`
	Usage       OIQUsageValues  `json:"usage"`
}

// OIQUsageValues holds the actual usage numbers.
// oiq uses "operations" for request-based and "data" for storage-based.
type OIQUsageValues struct {
	Operations *MinMaxPrice `json:"operations,omitempty"`
	Data       *MinMaxPrice `json:"data,omitempty"`
}

type flatResource struct {
	Address  string
	Type     string
	Change   string
	Before   MinMaxPrice
	After    MinMaxPrice
	Products []flatProduct
}

type flatProduct struct {
	Description string
	Price       MinMaxPrice
	UsageAmount string // human-readable usage, e.g. "55 GB" or "16,000,000 ops"
}

// EnvOIQResult groups OIQ results for a single environment.
type EnvOIQResult struct {
	Environment string
	Services    map[string][]byte // service name -> oiq price JSON
}

// FormatOIQResultsMultiEnv formats results across one or more environments.
func FormatOIQResultsMultiEnv(envResults []EnvOIQResult) (string, error) {
	var b strings.Builder

	b.WriteString("\n\nMonthly Cost Estimate using OpenInfraQuote\n")
	b.WriteString(strings.Repeat("═", 72) + "\n\n")

	var grandTotalBefore, grandTotalAfter MinMaxPrice

	for _, env := range envResults {
		envSection, envBefore, envAfter, err := formatSingleEnv(env.Environment, env.Services)
		if err != nil {
			return "", fmt.Errorf("failed to format environment %s: %w", env.Environment, err)
		}

		grandTotalBefore.Min += envBefore.Min
		grandTotalBefore.Max += envBefore.Max
		grandTotalAfter.Min += envAfter.Min
		grandTotalAfter.Max += envAfter.Max

		b.WriteString(envSection)
	}

	// Grand total only when multiple environments are selected
	if len(envResults) > 1 {
		b.WriteString(strings.Repeat("═", 72) + "\n")
		b.WriteString("  Total across all environments\n\n")

		diffMin := grandTotalAfter.Min - grandTotalBefore.Min
		diffMax := grandTotalAfter.Max - grandTotalBefore.Max

		if floatEq(diffMin, 0) && floatEq(diffMax, 0) {
			b.WriteString("  No change in monthly cost\n")
		} else if diffMin > 0 || diffMax > 0 {
			b.WriteString(fmt.Sprintf("  Monthly cost increase: +%s\n", formatRange(diffMin, diffMax)))
		} else {
			b.WriteString(fmt.Sprintf("  Monthly cost decrease: %s\n", formatRange(diffMin, diffMax)))
		}

		b.WriteString(fmt.Sprintf("  Before: %s\n", formatRange(grandTotalBefore.Min, grandTotalBefore.Max)))
		b.WriteString(fmt.Sprintf("  After:  %s\n", formatRange(grandTotalAfter.Min, grandTotalAfter.Max)))
	}

	return b.String(), nil
}

// formatSingleEnv formats one environment and returns its section string
// plus the environment-level before/after totals.
func formatSingleEnv(envName string, services map[string][]byte) (string, MinMaxPrice, MinMaxPrice, error) {
	var allResources []flatResource
	var totalBefore, totalAfter MinMaxPrice

	for _, jsonBytes := range services {
		var output OIQPriceOutput
		if err := json.Unmarshal(jsonBytes, &output); err != nil {
			return "", MinMaxPrice{}, MinMaxPrice{}, fmt.Errorf("failed to parse oiq output: %w", err)
		}

		totalBefore.Min += output.PrevPrice.Min
		totalBefore.Max += output.PrevPrice.Max
		totalAfter.Min += output.Price.Min
		totalAfter.Max += output.Price.Max

		for _, res := range output.Resources {
			fr := flatResource{
				Address: res.Address,
				Type:    res.Type,
				Change:  res.Change,
			}

			switch res.Change {
			case "add":
				fr.Before = MinMaxPrice{Min: 0, Max: 0}
				fr.After = res.Price
			case "delete":
				fr.Before = res.Price
				fr.After = MinMaxPrice{Min: 0, Max: 0}
			default:
				fr.Before = res.Price
				fr.After = res.Price
			}

			// Parse product-level usage breakdown
			for _, p := range res.Products {
				fp := flatProduct{
					Description: p.Usage.Description,
					Price:       p.Price,
					UsageAmount: formatUsageAmount(p.Usage.Usage),
				}
				fr.Products = append(fr.Products, fp)
			}

			allResources = append(allResources, fr)
		}
	}

	var added, removed, existing []flatResource
	for _, r := range allResources {
		switch r.Change {
		case "add":
			added = append(added, r)
		case "delete":
			removed = append(removed, r)
		default:
			existing = append(existing, r)
		}
	}

	var b strings.Builder

	b.WriteString(fmt.Sprintf("── %s ──\n\n", envName))

	diffMin := totalAfter.Min - totalBefore.Min
	diffMax := totalAfter.Max - totalBefore.Max

	if floatEq(diffMin, 0) && floatEq(diffMax, 0) {
		b.WriteString("  No change in monthly cost\n")
	} else if diffMin > 0 || diffMax > 0 {
		b.WriteString(fmt.Sprintf("  Monthly cost increase: +%s\n", formatRange(diffMin, diffMax)))
	} else {
		b.WriteString(fmt.Sprintf("  Monthly cost decrease: %s\n", formatRange(diffMin, diffMax)))
	}

	b.WriteString(fmt.Sprintf("  Before: %s\n", formatRange(totalBefore.Min, totalBefore.Max)))
	b.WriteString(fmt.Sprintf("  After:  %s\n\n", formatRange(totalAfter.Min, totalAfter.Max)))

	b.WriteString(fmt.Sprintf("  🟢 Added:     %d\n", len(added)))
	b.WriteString(fmt.Sprintf("  🔴 Removed:   %d\n", len(removed)))
	b.WriteString(fmt.Sprintf("  ⚪ Existing:  %d\n", len(existing)))

	if len(added) > 0 {
		b.WriteString("\n  Added resources:\n")
		writeResourceTable(&b, added)
	}
	if len(removed) > 0 {
		b.WriteString("\n  Removed resources:\n")
		writeResourceTable(&b, removed)
	}
	if len(existing) > 0 {
		b.WriteString("\n  Existing resources:\n")
		writeResourceTable(&b, existing)
	}

	b.WriteString("\n")

	return b.String(), totalBefore, totalAfter, nil
}

func writeResourceTable(b *strings.Builder, resources []flatResource) {
	addrW := len("Resource")
	typeW := len("Type")
	beforeW := len("Before")
	afterW := len("After")

	type row struct {
		addr, typ, before, after string
	}

	rows := make([]row, len(resources))
	for i, r := range resources {
		rows[i] = row{
			addr:   r.Address,
			typ:    r.Type,
			before: formatRange(r.Before.Min, r.Before.Max),
			after:  formatRange(r.After.Min, r.After.Max),
		}
		if len(rows[i].addr) > addrW {
			addrW = len(rows[i].addr)
		}
		if len(rows[i].typ) > typeW {
			typeW = len(rows[i].typ)
		}
		if len(rows[i].before) > beforeW {
			beforeW = len(rows[i].before)
		}
		if len(rows[i].after) > afterW {
			afterW = len(rows[i].after)
		}
	}

	const maxAddrW = 55
	if addrW > maxAddrW {
		addrW = maxAddrW
	}

	fmtStr := fmt.Sprintf("  %%-%ds  %%-%ds  %%%ds  %%%ds\n", addrW, typeW, beforeW, afterW)

	b.WriteString(fmt.Sprintf(fmtStr, "Resource", "Type", "Before", "After"))
	b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
		strings.Repeat("─", addrW),
		strings.Repeat("─", typeW),
		strings.Repeat("─", beforeW),
		strings.Repeat("─", afterW),
	))

	for i, r := range rows {
		addr := r.addr
		if len(addr) > maxAddrW {
			addr = "…" + addr[len(addr)-(maxAddrW-1):]
		}
		b.WriteString(fmt.Sprintf(fmtStr, addr, r.typ, r.before, r.after))

		// Write product breakdown under each resource
		products := resources[i].Products
		// Filter out zero-price products with no usage to keep it clean
		var visibleProducts []flatProduct
		for _, p := range products {
			if !floatEq(p.Price.Min, 0) || !floatEq(p.Price.Max, 0) || p.UsageAmount != "" {
				visibleProducts = append(visibleProducts, p)
			}
		}

		for j, p := range visibleProducts {
			var connector string
			if j == len(visibleProducts)-1 {
				connector = "└─"
			} else {
				connector = "├─"
			}

			usagePart := ""
			if p.UsageAmount != "" {
				usagePart = fmt.Sprintf(" (%s)", p.UsageAmount)
			}

			b.WriteString(fmt.Sprintf("    %s %s%s: %s\n",
				connector,
				p.Description,
				usagePart,
				formatRange(p.Price.Min, p.Price.Max),
			))
		}
	}
}

// formatUsageAmount turns OIQUsageValues into a human-readable string.
func formatUsageAmount(u OIQUsageValues) string {
	if u.Operations != nil {
		return fmt.Sprintf("%s ops", formatNumber(u.Operations.Min))
	}
	if u.Data != nil {
		return fmt.Sprintf("%s GB", formatNumber(u.Data.Min))
	}
	return ""
}

// formatNumber formats a number with commas for readability.
func formatNumber(v float64) string {
	if v == 0 {
		return "0"
	}

	// If it's a whole number, format without decimals
	isWhole := v == math.Floor(v)

	var s string
	if isWhole {
		s = fmt.Sprintf("%.0f", v)
	} else {
		s = fmt.Sprintf("%.2f", v)
	}

	// Insert commas
	parts := strings.Split(s, ".")
	intPart := parts[0]

	negative := false
	if intPart[0] == '-' {
		negative = true
		intPart = intPart[1:]
	}

	var result []byte
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}

	if negative {
		result = append([]byte{'-'}, result...)
	}

	if len(parts) > 1 {
		return string(result) + "." + parts[1]
	}
	return string(result)
}

func formatRange(min, max float64) string {
	if floatEq(min, max) {
		return formatUSD(min)
	}
	return fmt.Sprintf("%s - %s", formatUSD(min), formatUSD(max))
}

func formatUSD(v float64) string {
	if v < 0 {
		return fmt.Sprintf("-$%.2f", math.Abs(v))
	}
	return fmt.Sprintf("$%.2f", v)
}

func floatEq(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}