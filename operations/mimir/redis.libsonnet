local redis = import 'redis/redis.libsonnet';

redis {
  redis+:: {
    _config+:: {
      namespace: $._config.namespace,
      replicas: 1,
    },
  },
  // Dedicated memcached instance used to cache query results.
  redis_frontend: $.redis {
    name: 'redis-frontend',
    max_item_size: '5m',
  },
  // Dedicated redis instance used to temporarily cache index lookups.
  redis_index_queries:
  if $._config.cache_index_queries_enabled && $._config.cache_index_queries_backend == 'redis' then
    $.redis {
      name: 'redis-index-queries',
      max_item_size: '%dm' % [$._config.cache_index_queries_max_item_size_mb],
      connection_limit: 16384,
    }
  else {},

  // Redis instance used to cache chunks.
  redis_chunks:
  if $._config.cache_chunks_enabled && $._config.cache_chunks_backend == 'redis' then
    $.redis {
      name: 'redis',
      max_item_size: '%dm' % [$._config.cache_chunks_max_item_size_mb],

      // Save memory by more tightly provisioning redis chunks.
      memory_limit_mb: 6 * 1024,
      overprovision_factor: 1.05,
      connection_limit: 16384,

      local container = $.core.v1.container,
    }
  else {},

  // Memcached instance for caching TSDB blocks metadata (meta.json files, deletion marks, list of users and blocks).
  redis_metadata:
  if $._config.cache_metadata_enabled && $._config.cache_metadata_backend == 'redis' then
    $.redis {
      name: 'redis-metadata',
      max_item_size: '%dm' % [$._config.cache_metadata_max_item_size_mb],
      connection_limit: 16384,

      // Metadata cache doesn't need much memory.
      memory_limit_mb: 512,

      local statefulSet = $.apps.v1.statefulSet,
      statefulSet+:
        statefulSet.mixin.spec.withReplicas(1),
    },
}

