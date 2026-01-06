FROM python:3.11-slim

WORKDIR /app

# 安装系统依赖 (libgl1 替代了已废弃的 libgl1-mesa-glx)
RUN apt-get update && apt-get install -y \
    libgl1 \
    libglib2.0-0 \
    libsm6 \
    libxext6 \
    libxrender1 \
    && rm -rf /var/lib/apt/lists/*

# 复制依赖文件
COPY ai_service/requirements.txt .

# 安装 Python 依赖
RUN pip install --no-cache-dir -r requirements.txt

# 先生成 proto 代码（在复制源代码之前）
COPY proto /tmp/proto
RUN mkdir -p pb && \
    python -m grpc_tools.protoc \
    --proto_path=/tmp/proto \
    --python_out=pb \
    --pyi_out=pb \
    --grpc_python_out=pb \
    /tmp/proto/*.proto && \
    touch pb/__init__.py && \
    sed -i 's/^import detection_pb2/from . import detection_pb2/' pb/detection_pb2_grpc.py && \
    rm -rf /tmp/proto && \
    echo "=== Generated pb files ===" && ls -la pb/

# 复制源代码（pb目录已存在，不会被覆盖）
COPY ai_service/*.py ./
COPY ai_service/models ./models/

# 验证 pb 目录和文件
RUN echo "=== Verifying pb directory ===" && \
    ls -la pb/ && \
    python -c "import pb.detection_pb2; import pb.detection_pb2_grpc; print('✓ Proto imports successful')"

# 暴露端口
EXPOSE 50051

# 启动服务
CMD ["python", "server.py", "--port", "50051"]
