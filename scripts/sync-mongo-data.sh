#!/bin/bash
# ============================================
# MongoDB 数据同步脚本
# 功能: 将 mongo-init.json 自动导入到 K8s MongoDB
# 用法: ./sync-mongo-data.sh [JSON文件路径] [命名空间] [数据库名]
# ============================================
set -e

JSON_FILE="${1:-/Volumes/data/rsync/kube-nova/scripts/mongo-init.json}"
NAMESPACE="${2:-kube-nova}"
MONGO_DB="${3:-kube_nova_devops}"
MONGO_USER="${MONGO_USER:-root}"
MONGO_PASSWORD="${MONGO_PASSWORD:-8VlZ2lvIsKBCYSE3}"

RED='\033[0;31m'; GREEN='\033[0;32m'; CYAN='\033[0;36m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

echo ""
echo -e "${CYAN}============================================${NC}"
echo -e "${CYAN}  MongoDB 数据同步到 K8s 集群${NC}"
echo -e "${CYAN}============================================${NC}"
echo ""

[ ! -f "$JSON_FILE" ] && error "JSON 文件不存在: $JSON_FILE"
info "数据文件: $JSON_FILE ($(du -sh "$JSON_FILE" | cut -f1))"

# 自动获取 MongoDB Pod 名
MONGO_POD=$(kubectl get pod -n "$NAMESPACE" -l app=mongodb -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
[ -z "$MONGO_POD" ] && error "未找到 MongoDB Pod (ns=$NAMESPACE, label=app=mongodb)"
info "MongoDB Pod: $MONGO_POD"

MONGO_URI="mongodb://${MONGO_USER}:${MONGO_PASSWORD}@mongodb.${NAMESPACE}.svc.cluster.local:27017/?authSource=admin"
info "目标数据库: $MONGO_DB"
echo ""

python3 << PYEOF
import json, subprocess, os, tempfile
json_file = "$JSON_FILE"
ns = "$NAMESPACE"; pod = "$MONGO_POD"; uri = "$MONGO_URI"; db = "$MONGO_DB"
with open(json_file) as f:
    data = json.load(f)
total = 0; imported = 0
for coll, docs in data.get("collections", {}).items():
    if not docs:
        continue
    with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as tf:
        json.dump(docs, tf); tmp = tf.name
    pp = f"/tmp/{coll}.json"
    subprocess.run(["kubectl", "cp", tmp, f"{ns}/{pod}:{pp}"], capture_output=True)
    os.unlink(tmp)
    r = subprocess.run(["kubectl", "exec", "-n", ns, pod, "--", "mongoimport",
        f"--uri={uri}", f"--db={db}", f"--collection={coll}", f"--file={pp}",
        "--drop", "--jsonArray"], capture_output=True, text=True)
    if r.returncode == 0:
        print(f"  \033[32m✓\033[0m {coll}: {len(docs)} docs"); imported += 1
    else:
        print(f"  \033[31m✗\033[0m {coll}: {r.stderr.strip()}")
    total += len(docs)
print(f"\n导入完成: {total} 条文档, {imported} 个集合 → {db}")
PYEOF

echo ""
info "同步完成!"
