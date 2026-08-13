#!/bin/bash
sed -i 's/"includeRedis": true,/"subcomponents": ["redis", "postgres"],/g' README.md
sed -i '/"includePostgres": false/d' README.md
