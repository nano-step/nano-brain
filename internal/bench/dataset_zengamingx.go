package bench

const ZengamingxDatasetVersion = "v1"

func ZengamingxDataset() *BenchmarkDataset {
	return &BenchmarkDataset{
		Version:       ZengamingxDatasetVersion,
		Scale:         len(zengamingxQueries),
		WorkspaceHash: "d1915ee19311546a064576fc5df565da7ab20fe1c4a81c97e3ba6e9059d977b7",
		Entries:       zengamingxQueries,
	}
}

var zengamingxQueries = []DatasetEntry{
	{
		Query: "item prices service",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/item-prices/item-prices.service.ts",
			"csgoskins-pricing/src/item-prices/item-prices.repo.ts",
		},
	},
	{
		Query: "redis lock service",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/common/redis/lock.service.ts",
			"csgoskins-pricing/src/common/redis/redis.service.ts",
		},
	},
	{
		Query: "page extractor service",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/page-extractor/page-extractor.service.ts",
			"csgoskins-pricing/src/page-extractor/page-extractor.extractors.ts",
		},
	},
	{
		Query: "aws s3 service",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/aws-s3/aws-s3.service.ts",
		},
	},
	{
		Query: "health controller",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/health/health.controller.ts",
		},
	},
	{
		Query: "http config service",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/common/http-config/http-config.service.ts",
		},
	},
	{
		Query: "item prices entity",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/item-prices/item-prices.entity.ts",
		},
	},
	{
		Query: "logging decorator",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/utils/logging/logging.decorator.ts",
			"csgoskins-pricing/src/utils/logging/logging.helper.ts",
		},
	},
	{
		Query: "backpack tf constants",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/common/backpack-tf.const.ts",
		},
	},
	{
		Query: "currency interface",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/common/interfaces/currency.interface.ts",
		},
	},
	{
		Query: "items dto",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/dto/items.dto.ts",
		},
	},
	{
		Query: "app module nestjs",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/app.module.ts",
			"csgoskins-pricing/src/main.ts",
		},
	},
	{
		Query: "redis module",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/common/redis/redis.module.ts",
		},
	},
	{
		Query: "helpers utils",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/utils/helpers.ts",
		},
	},
	{
		Query: "enums common",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/common/enums.ts",
		},
	},
	{
		Query: "class with logger",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/common/class-with-logger.ts",
		},
	},
	{
		Query: "item prices module",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/item-prices/item-prices.module.ts",
		},
	},
	{
		Query: "item prices log update dto",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/dto/item-prices-log-update.dto.ts",
		},
	},
	{
		Query: "redis decorator",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/common/redis/redis.decorator.ts",
		},
	},
	{
		Query: "page extractor module",
		RelevantSourcePaths: []string{
			"csgoskins-pricing/src/page-extractor/page-extractor.module.ts",
		},
	},
}
