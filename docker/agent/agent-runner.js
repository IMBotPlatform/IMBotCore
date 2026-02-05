/**
 * Agent Runner for IMBotCore Container
 * 从 stdin 读取 JSON 输入，执行 Claude Code，输出 JSON 结果
 */
const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');

// 输出标记用于健壮的 JSON 解析
const OUTPUT_START_MARKER = '---IMBOTCORE_OUTPUT_START---';
const OUTPUT_END_MARKER = '---IMBOTCORE_OUTPUT_END---';

/**
 * 从 stdin 读取完整输入
 */
async function readInput() {
    return new Promise((resolve, reject) => {
        let data = '';
        process.stdin.setEncoding('utf8');
        process.stdin.on('data', chunk => data += chunk);
        process.stdin.on('end', () => {
            try {
                resolve(JSON.parse(data));
            } catch (err) {
                reject(new Error(`Invalid JSON input: ${err.message}`));
            }
        });
        process.stdin.on('error', reject);
    });
}

/**
 * 加载环境变量文件
 */
function loadEnvFile() {
    const envFile = '/workspace/env-dir/env';
    if (fs.existsSync(envFile)) {
        const content = fs.readFileSync(envFile, 'utf-8');
        for (const line of content.split('\n')) {
            const trimmed = line.trim();
            if (!trimmed || trimmed.startsWith('#')) continue;
            const idx = trimmed.indexOf('=');
            if (idx > 0) {
                const key = trimmed.slice(0, idx);
                const value = trimmed.slice(idx + 1);
                process.env[key] = value;
            }
        }
    }
}

/**
 * 输出 JSON 结果
 */
function output(result) {
    console.log(OUTPUT_START_MARKER);
    console.log(JSON.stringify(result));
    console.log(OUTPUT_END_MARKER);
}

/**
 * 执行 Claude Code
 */
async function runClaudeCode(input) {
    const { prompt, sessionId, isNewSession, chatId } = input;

    const args = ['--print'];

    // Session 管理
    if (sessionId) {
        if (isNewSession) {
            args.push('--session-id', sessionId);
        } else {
            args.push('--resume', sessionId);
        }
    }

    // 工作目录
    args.push('--output-format', 'text');
    args.push(prompt);

    return new Promise((resolve, reject) => {
        const child = spawn('claude', args, {
            cwd: '/workspace/group',
            env: process.env,
            stdio: ['pipe', 'pipe', 'pipe']
        });

        let stdout = '';
        let stderr = '';

        child.stdout.on('data', data => stdout += data.toString());
        child.stderr.on('data', data => stderr += data.toString());

        child.on('close', code => {
            if (code !== 0) {
                reject(new Error(`Claude exited with code ${code}: ${stderr.slice(-200)}`));
            } else {
                resolve(stdout.trim());
            }
        });

        child.on('error', reject);
    });
}

/**
 * 主函数
 */
async function main() {
    try {
        // 加载环境变量
        loadEnvFile();

        // 读取输入
        const input = await readInput();

        // 执行 Claude
        const result = await runClaudeCode(input);

        // 输出成功结果
        output({
            status: 'success',
            result: result,
            newSessionId: input.sessionId || null
        });

        process.exit(0);
    } catch (err) {
        // 输出错误结果
        output({
            status: 'error',
            result: null,
            error: err.message
        });

        process.exit(1);
    }
}

main();
