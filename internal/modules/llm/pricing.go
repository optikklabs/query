package llm

import "github.com/ClickHouse/clickhouse-go/v2"

// modelPrice is USD per 1M tokens; cost is derived at query time so the
// table can be re-priced without any backfill.
type modelPrice struct {
	In  float64
	Out float64
}

var priceTable = map[string]modelPrice{
	"gpt-4o":                 {In: 2.50, Out: 10.00},
	"gpt-4o-mini":            {In: 0.15, Out: 0.60},
	"gpt-4.1":                {In: 2.00, Out: 8.00},
	"gpt-4.1-mini":           {In: 0.40, Out: 1.60},
	"o3":                     {In: 2.00, Out: 8.00},
	"claude-sonnet-4-5":      {In: 3.00, Out: 15.00},
	"claude-opus-4-1":        {In: 15.00, Out: 75.00},
	"claude-haiku-4-5":       {In: 1.00, Out: 5.00},
	"gemini-1.5-pro":         {In: 1.25, Out: 5.00},
	"gemini-2.0-flash":       {In: 0.10, Out: 0.40},
	"text-embedding-3-small": {In: 0.02, Out: 0},
	"text-embedding-3-large": {In: 0.13, Out: 0},
}

// priceArgs exposes the table to SQL as parallel arrays consumed by
// transform(model, @priceModels, @priceIn/@priceOut, 0.).
func priceArgs() []any {
	models := make([]string, 0, len(priceTable))
	in := make([]float64, 0, len(priceTable))
	out := make([]float64, 0, len(priceTable))
	for m, p := range priceTable {
		models = append(models, m)
		in = append(in, p.In)
		out = append(out, p.Out)
	}
	return []any{
		clickhouse.Named("priceModels", models),
		clickhouse.Named("priceIn", in),
		clickhouse.Named("priceOut", out),
	}
}

// tokenCostSQL prices one row's tokens; column names vary per query.
func tokenCostSQL(inCol, outCol, modelCol string) string {
	return "(" + inCol + " * transform(" + modelCol + ", @priceModels, @priceIn, 0.) + " +
		outCol + " * transform(" + modelCol + ", @priceModels, @priceOut, 0.)) / 1e6"
}

func costOf(model string, in, out uint64) float64 {
	p, ok := priceTable[model]
	if !ok {
		return 0
	}
	return (float64(in)*p.In + float64(out)*p.Out) / 1e6
}
