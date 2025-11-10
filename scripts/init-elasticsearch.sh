#!/bin/bash

# Elasticsearch initialization script
# Creates index templates and ILM policies for Bananas logs

set -e

ES_HOST="${ES_HOST:-http://localhost:9200}"
INDEX_PREFIX="${INDEX_PREFIX:-bananas-logs}"

echo "Initializing Elasticsearch at $ES_HOST..."

# Wait for Elasticsearch to be ready
echo "Waiting for Elasticsearch to be healthy..."
for i in {1..30}; do
    if curl -s -f "$ES_HOST/_cluster/health" > /dev/null 2>&1; then
        echo "Elasticsearch is ready!"
        break
    fi
    echo "Waiting... ($i/30)"
    sleep 2
done

# Create ILM policy for log retention
echo "Creating ILM policy for log retention..."
curl -X PUT "$ES_HOST/_ilm/policy/bananas-logs-policy" \
  -H 'Content-Type: application/json' \
  -d '{
    "policy": {
      "phases": {
        "hot": {
          "min_age": "0ms",
          "actions": {
            "rollover": {
              "max_primary_shard_size": "50GB",
              "max_age": "1d"
            },
            "set_priority": {
              "priority": 100
            }
          }
        },
        "warm": {
          "min_age": "7d",
          "actions": {
            "shrink": {
              "number_of_shards": 1
            },
            "forcemerge": {
              "max_num_segments": 1
            },
            "set_priority": {
              "priority": 50
            }
          }
        },
        "cold": {
          "min_age": "30d",
          "actions": {
            "set_priority": {
              "priority": 0
            }
          }
        },
        "delete": {
          "min_age": "90d",
          "actions": {
            "delete": {}
          }
        }
      }
    }
  }'

echo ""
echo "ILM policy created successfully!"

# Create index template for Bananas logs
echo "Creating index template..."
curl -X PUT "$ES_HOST/_index_template/bananas-logs-template" \
  -H 'Content-Type: application/json' \
  -d '{
    "index_patterns": ["'"$INDEX_PREFIX"'-*"],
    "template": {
      "settings": {
        "number_of_shards": 1,
        "number_of_replicas": 0,
        "index.lifecycle.name": "bananas-logs-policy",
        "index.lifecycle.rollover_alias": "'"$INDEX_PREFIX"'",
        "refresh_interval": "5s"
      },
      "mappings": {
        "properties": {
          "timestamp": {
            "type": "date",
            "format": "strict_date_optional_time||epoch_millis"
          },
          "level": {
            "type": "keyword"
          },
          "message": {
            "type": "text",
            "fields": {
              "keyword": {
                "type": "keyword",
                "ignore_above": 256
              }
            }
          },
          "component": {
            "type": "keyword"
          },
          "log_source": {
            "type": "keyword"
          },
          "job_id": {
            "type": "keyword"
          },
          "worker_id": {
            "type": "keyword"
          },
          "error": {
            "type": "text",
            "fields": {
              "keyword": {
                "type": "keyword",
                "ignore_above": 512
              }
            }
          },
          "fields": {
            "type": "object",
            "enabled": true
          }
        }
      }
    },
    "priority": 500,
    "composed_of": [],
    "version": 1,
    "_meta": {
      "description": "Index template for Bananas distributed task queue logs"
    }
  }'

echo ""
echo "Index template created successfully!"

# Create initial index with alias
echo "Creating initial index..."
TODAY=$(date +%Y.%m.%d)
INITIAL_INDEX="${INDEX_PREFIX}-${TODAY}-000001"

curl -X PUT "$ES_HOST/$INITIAL_INDEX" \
  -H 'Content-Type: application/json' \
  -d '{
    "aliases": {
      "'"$INDEX_PREFIX"'": {
        "is_write_index": true
      }
    }
  }'

echo ""
echo "Initial index created: $INITIAL_INDEX"

# Verify setup
echo ""
echo "Verifying setup..."
curl -s "$ES_HOST/_cat/indices/${INDEX_PREFIX}-*?v"

echo ""
echo "✅ Elasticsearch initialization complete!"
echo ""
echo "Kibana UI: http://localhost:5601"
echo "Elasticsearch API: $ES_HOST"
echo ""
echo "You can now configure Bananas to use Elasticsearch:"
echo "  LOG_ES_ENABLED=true"
echo "  LOG_ES_ADDRESSES=$ES_HOST"
echo "  LOG_ES_INDEX_PREFIX=$INDEX_PREFIX"
