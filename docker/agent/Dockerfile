# Claude Code Agent Container
# 用于隔离执行 AI Agent 的容器镜像
FROM node:22-slim

LABEL maintainer="IMBotPlatform"
LABEL description="Claude Code Agent Container for IMBotCore"

# 设置工作目录
WORKDIR /workspace

# 安装必要依赖
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# 安装 Claude Code CLI
RUN npm install -g @anthropic-ai/claude-code

# 创建运行用户
RUN useradd -m -s /bin/bash agent \
    && mkdir -p /workspace/group /workspace/ipc /workspace/global \
    && chown -R agent:agent /workspace

# 切换到非 root 用户
USER agent

# 创建 Claude 配置目录
RUN mkdir -p /home/agent/.claude

# 设置环境变量
ENV HOME=/home/agent
ENV NODE_ENV=production

# 工作目录
WORKDIR /workspace/group

# 复制 agent-runner 脚本
COPY --chown=agent:agent agent-runner.js /workspace/

# 入口点
ENTRYPOINT ["node", "/workspace/agent-runner.js"]
