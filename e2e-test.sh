#!/bin/bash

# Cleanup existing container if it exists
if [ "$(docker ps -aq -f name=opensearch-test)" ]; then
    echo "Removing existing opensearch-test container..."
    docker rm -f opensearch-test
fi

# Generate a random initial admin password
PASSWORD=$(openssl rand -base64 12)

# Start OpenSearch in Docker
docker run -d --name opensearch-test -p 9200:9200 \
  -e "discovery.type=single-node" \
  -e "OPENSEARCH_INITIAL_ADMIN_PASSWORD=$PASSWORD" \
  opensearchproject/opensearch:latest

# Wait for OpenSearch to be ready
echo "Waiting for OpenSearch to be ready..."
until curl -s --insecure -u admin:$PASSWORD "https://localhost:9200/_cluster/health?wait_for_status=yellow&timeout=60s" | grep -qE '"status":"(green|yellow)"'; do
    echo "Waiting for OpenSearch... (retrying in 5s)"
    sleep 5
done
echo "OpenSearch is ready."

# Create mapping file to avoid shell escaping issues
cat <<EOF > mapping.json
{
  "mappings": {
    "properties": {
      "message": {"type": "text"},
      "timestamp": {"type": "date"},
      "level": {"type": "keyword"}
    }
  }
}
EOF

# Create an index for logs
echo "Creating index 'logs'..."
curl -X PUT "https://localhost:9200/logs" \
  -H 'Content-Type: application/json' \
  -u admin:$PASSWORD \
  --insecure \
  -d @mapping.json

rm mapping.json

# Populate the index with random log-like data
echo "Populating index with sample log data..."
for i in {1..20}; do
  LEVELS=("INFO" "WARN" "ERROR" "DEBUG")
  LEVEL=${LEVELS[$RANDOM % ${#LEVELS[@]}]}
  MESSAGE="Sample log message $i: This is a test log entry with level $LEVEL"
  TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  
  # Use a temporary file for the doc to ensure safety as well
  echo "{\"message\": \"$MESSAGE\", \"timestamp\": \"$TIMESTAMP\", \"level\": \"$LEVEL\"}" > doc.json

  curl -X POST "https://localhost:9200/logs/_doc" \
    -H 'Content-Type: application/json' \
    -u admin:$PASSWORD \
    --insecure \
    -d @doc.json \
    >/dev/null 2>&1
    
  rm doc.json
done

echo "OpenSearch e2e test setup complete."
echo "OpenSearch is running on https://localhost:9200"
echo "Initial admin password: $PASSWORD"
echo "To stop: docker stop opensearch-test && docker rm opensearch-test"
