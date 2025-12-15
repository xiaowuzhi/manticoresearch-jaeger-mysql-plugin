#!/bin/bash
# ManticoreSearch 数据导出脚本
# 使用方法: ./export-manticore-data.sh <table_name> <output_format>

TABLE_NAME=${1:-users}
FORMAT=${2:-csv}  # csv, json, sql

NAMESPACE="tracing"
SERVICE="manticore"

echo "📊 导出 ManticoreSearch 表: $TABLE_NAME"
echo "格式: $FORMAT"
echo ""

# 查询数据
QUERY="SELECT * FROM $TABLE_NAME"
RESULT=$(kubectl exec -n $NAMESPACE deployment/$SERVICE -- sh -c \
  "curl -s 'http://localhost:9308/sql' -d 'mode=raw&query=$QUERY'")

if [ "$FORMAT" == "csv" ]; then
    echo "$RESULT" | jq -r '.[0].columns | keys | @csv' > ${TABLE_NAME}.csv
    echo "$RESULT" | jq -r '.[0].data[] | [.[]] | @csv' >> ${TABLE_NAME}.csv
    echo "✅ CSV 文件已导出: ${TABLE_NAME}.csv"
    
elif [ "$FORMAT" == "json" ]; then
    echo "$RESULT" > ${TABLE_NAME}.json
    echo "✅ JSON 文件已导出: ${TABLE_NAME}.json"
    
elif [ "$FORMAT" == "sql" ]; then
    # 生成 INSERT 语句
    echo "-- ManticoreSearch 表导出: $TABLE_NAME" > ${TABLE_NAME}.sql
    echo "-- 导出时间: $(date)" >> ${TABLE_NAME}.sql
    echo "" >> ${TABLE_NAME}.sql
    
    # 获取列名
    COLUMNS=$(echo "$RESULT" | jq -r '.[0].columns | keys | join(",")')
    
    # 生成 INSERT 语句
    echo "$RESULT" | jq -r --arg cols "$COLUMNS" \
      '.[0].data[] | "INSERT INTO '${TABLE_NAME}' (\($cols)) VALUES (\(to_entries | map(.value | tostring) | join(",")));"' >> ${TABLE_NAME}.sql
    
    echo "✅ SQL 文件已导出: ${TABLE_NAME}.sql"
fi

echo ""
echo "📁 文件位置: $(pwd)/${TABLE_NAME}.${FORMAT}"



