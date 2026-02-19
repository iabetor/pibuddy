# 声纹识别技术设计文档

## Context

PiBuddy 是一个 Go 语言开发的树莓派语音助手,当前架构为单线程 Pipeline 模式。需要添加声纹识别能力以支持多用户场景,但必须满足：
- 树莓派资源受限(4GB 内存,ARM64 CPU)
- 不能阻塞主对话流程
- Go 生态缺少成熟的声纹识别库
- 需要高准确率(错误率 <15%)
- 追求极致性能(启动快、内存占用低)

## Goals / Non-Goals

**Goals**:
- 实现稳定的声纹识别能力(准确率 >85%)
- 识别延迟 <1.5 秒(相比Python方案提升60%)
- 内存占用 <150MB(相比Python方案减少70%)
- 支持至少 5 个用户注册
- 推理引擎独立可测试

**Non-Goals**:
- 实时声纹追踪(对话中持续验证)
- 防欺骗攻击(录音回放检测)
- 情绪识别和年龄识别
- 云端识别服务
- 自定义模型训练

## Decisions

### 决策 1: Rust + ONNX Runtime 方案(方案A)

**选择**: Rust推理库 + 预训练ONNX模型 + Go FFI调用

**理由**:
- **性能极佳**: Rust零成本抽象,内存占用仅100MB,启动时间<100ms
- **预训练模型可用**: HuggingFace提供多个ONNX格式声纹模型(pyannote/embedding, wespeaker等)
- **无运行时依赖**: 编译为动态库(.so),无需Python环境
- **树莓派优化**: 支持ARM64 NEON指令集,可应用量化优化
- **开发周期短**: 跳过模型导出步骤,直接使用成熟模型

**替代方案**:
- ❌ **Python + gRPC**: 内存500MB,启动2-3秒,需要独立进程管理
- ❌ **纯Go实现(Govpr)**: 准确率低(~30%错误率),生态不成熟
- ❌ **云端API**: 延迟高(>5s),依赖网络,隐私风险

**方案对比**:
| 指标 | Rust+ONNX | Python+gRPC | 纯Go |
|------|-----------|-------------|------|
| 内存占用 | 100MB | 500MB | 150MB |
| 识别延迟 | 0.5-1s | 2-3s | 3-5s |
| 启动时间 | <100ms | 2-3s | 即时 |
| 准确率 | 85-90% | 85-90% | 60-70% |
| 部署复杂度 | 低(单文件) | 中(Python环境) | 低 |

### 决策 2: 使用预训练ONNX模型

**选择**: 直接下载HuggingFace的ONNX模型,优先选择`nevil-ramani/pyannote_embedding_onnx`

**模型信息**:
- **模型大小**: 17MB (原PyTorch版150MB,缩小88%)
- **输入格式**: 16kHz mono PCM, 动态长度(3-5秒)
- **输出维度**: 192维embedding向量
- **准确率**: DER 11-20%(与原模型一致)

**模型下载地址**:
```bash
# HuggingFace直接下载
wget https://huggingface.co/nevil-ramani/pyannote_embedding_onnx/resolve/main/pyannote_embedding.onnx

# 备选: wespeaker模型(支持中文优化)
wget https://huggingface.co/hbredin/wespeaker-voxceleb-resnet34-LM/resolve/main/pytorch_model.onnx
```

**理由**:
- ✅ 跳过Python模型导出步骤,降低技术复杂度
- ✅ 社区验证过的模型,稳定性好
- ✅ 支持Node.js和Python验证,方便调试
- ✅ MIT协议,无使用限制

**替代方案**:
- ❌ **自己导出ONNX**: 需要Python环境,增加2-3天工作量
- ❌ **混合方案(先Python后Rust)**: 前期Python验证后迁移,工作量翻倍

### 决策 3: Tract作为ONNX推理引擎

**选择**: 使用`tract-onnx` crate实现推理

**理由**:
- 纯Rust实现,无C++依赖
- 支持ONNX 1.4.1-1.5.0主要算子
- 专为嵌入式优化(支持流式音频)
- 活跃维护(最近更新2024年)
- MIT协议

**替代方案**:
- ❌ **ONNX Runtime C API + FFI**: 需要编译C++库,交叉编译困难
- ❌ **Sherpa-ONNX**: 功能全面但体积大(>50MB),对PiBuddy过度设计

### 决策 4: FFI集成到Go主程序

**选择**: Rust编译为动态库(.so), Go通过CGo调用

**接口设计**:
```rust
// Rust端导出C ABI
#[no_mangle]
pub extern "C" fn voiceprint_init(model_path: *const c_char) -> *mut VoiceprintEngine;

#[no_mangle]
pub extern "C" fn voiceprint_register(
    engine: *mut VoiceprintEngine,
    user_id: *const c_char,
    audio_data: *const f32,
    audio_len: usize
) -> i32;

#[no_mangle]
pub extern "C" fn voiceprint_identify(
    engine: *mut VoiceprintEngine,
    audio_data: *const f32,
    audio_len: usize,
    out_user_id: *mut c_char,
    out_confidence: *mut f32
) -> i32;

#[no_mangle]
pub extern "C" fn voiceprint_free(engine: *mut VoiceprintEngine);
```

```go
// Go端CGo调用
package voiceprint

/*
#cgo LDFLAGS: -L./lib -lvoiceprint -ldl -lm
#include "./lib/voiceprint.h"
*/
import "C"

type Engine struct {
    ptr *C.VoiceprintEngine
}

func NewEngine(modelPath string) (*Engine, error) {
    cPath := C.CString(modelPath)
    defer C.free(unsafe.Pointer(cPath))
    
    ptr := C.voiceprint_init(cPath)
    if ptr == nil {
        return nil, errors.New("failed to init engine")
    }
    return &Engine{ptr: ptr}, nil
}

func (e *Engine) Identify(audio []float32) (userID string, confidence float32, err error) {
    // 零拷贝传递音频数据
    var outID [64]C.char
    var outConf C.float
    
    ret := C.voiceprint_identify(
        e.ptr,
        (*C.float)(unsafe.Pointer(&audio[0])),
        C.size_t(len(audio)),
        &outID[0],
        &outConf,
    )
    
    if ret != 0 {
        return "", 0, errors.New("identification failed")
    }
    
    return C.GoString(&outID[0]), float32(outConf), nil
}
```

**理由**:
- 零拷贝音频数据传递(性能最优)
- 生命周期管理清晰(Go控制,Rust执行)
- 错误传播机制完善
- 可单独测试Rust库

### 决策 5: SQLite 本地存储用户数据

**选择**: SQLite存储用户ID、声纹特征向量(192维float数组)、元数据

**Schema设计**:
```sql
CREATE TABLE users (
    user_id TEXT PRIMARY KEY,
    embedding BLOB NOT NULL,  -- 192 * 4 bytes = 768 bytes
    created_at INTEGER NOT NULL,
    sample_count INTEGER DEFAULT 1
);

CREATE TABLE user_samples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    embedding BLOB NOT NULL,
    recorded_at INTEGER NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(user_id)
);
```

**理由**:
- 轻量级,无需额外服务
- BLOB类型存储embedding向量
- 支持多样本注册(提升准确率)
- 方便备份和迁移

## Architecture

### 系统组件图

```
┌──────────────────────────────────────────────────────────┐
│                   PiBuddy (Go)                           │
│  ┌───────────┐     ┌──────────────────┐                 │
│  │ Pipeline  │────>│ Voiceprint       │                 │
│  │ (Main)    │     │ Package (Go)     │                 │
│  └───────────┘     └────────┬─────────┘                 │
│                              │ CGo/FFI                    │
└──────────────────────────────┼──────────────────────────┘
                               │
┌──────────────────────────────┼──────────────────────────┐
│      libvoiceprint.so (Rust)                            │
│  ┌───────────────────────────┴────────────────┐         │
│  │          C ABI Wrapper                     │         │
│  └───────────────────┬────────────────────────┘         │
│                      │                                   │
│  ┌───────────────────┴────────────┐   ┌────────────┐   │
│  │  ONNX Inference Engine         │   │  Feature   │   │
│  │  (tract-onnx)                  │   │  Manager   │   │
│  └───────────────────┬────────────┘   └────────────┘   │
│                      │                                   │
│  ┌───────────────────┴────────────┐                     │
│  │  pyannote_embedding.onnx       │                     │
│  │  (17MB 预训练模型)              │                     │
│  └────────────────────────────────┘                     │
└──────────────────────────────────────────────────────────┘
       │
       └─────> SQLite Database (users.db)
```

### 识别流程时序图

```
User    Pipeline    VoiceprintPkg    RustEngine    ONNX    SQLite
 │          │              │              │          │         │
 │─唤醒─────>│              │              │          │         │
 │          │─采集音频(3s)─>│              │          │         │
 │          │              │─Identify()──>│          │         │
 │          │              │              │─推理────>│         │
 │          │              │              │<─embedding│         │
 │          │              │              │─查询────────────────>│
 │          │              │              │<─用户embeddings─────│
 │          │              │              │─余弦相似度计算────>│
 │          │              │<─UserID/0.85─│          │         │
 │          │<─UserContext─│              │          │         │
 │<─Hi,xxx!─│              │              │          │         │
```

### 数据流与内存布局

**音频数据流**:
```
麦克风 → Go []byte → []float32 → unsafe.Pointer → Rust *const f32 → ONNX
        (16bit PCM)  (归一化)     (零拷贝)         (推理)
```

**Embedding比较**:
```rust
// 余弦相似度计算(Rust端)
fn cosine_similarity(a: &[f32], b: &[f32]) -> f32 {
    let dot: f32 = a.iter().zip(b).map(|(x, y)| x * y).sum();
    let norm_a: f32 = a.iter().map(|x| x * x).sum::<f32>().sqrt();
    let norm_b: f32 = b.iter().map(|x| x * x).sum::<f32>().sqrt();
    dot / (norm_a * norm_b)
}

// 识别逻辑
fn identify_user(current_emb: &[f32], db_users: &[(String, Vec<f32>)], threshold: f32) 
    -> Option<(String, f32)> 
{
    db_users.iter()
        .map(|(id, emb)| (id.clone(), cosine_similarity(current_emb, emb)))
        .max_by(|a, b| a.1.partial_cmp(&b.1).unwrap())
        .filter(|(_, score)| *score >= threshold)
}
```

## Implementation Plan

### Phase 1: Rust推理引擎开发(3-4天)

**1.1 环境搭建(0.5天)**
```bash
# 创建Rust项目
cargo new --lib voiceprint-rs
cd voiceprint-rs

# 添加依赖
cargo add tract-onnx ndarray hound rusqlite

# 下载预训练模型
mkdir -p models
wget -O models/pyannote_embedding.onnx \
  https://huggingface.co/nevil-ramani/pyannote_embedding_onnx/resolve/main/pyannote_embedding.onnx
```

**1.2 ONNX推理实现(1天)**
- 加载ONNX模型
- 实现音频预处理(重采样、归一化)
- 提取embedding向量
- 单元测试(使用示例音频验证输出)

**1.3 用户管理与相似度计算(1天)**
- SQLite数据库封装
- 余弦相似度计算
- 多样本平均embedding
- 注册/识别接口实现

**1.4 C ABI封装(0.5天)**
- 导出C风格函数
- 错误码定义
- 内存安全处理(panic处理)
- 编写C头文件

**1.5 测试与优化(1天)**
- Rust单元测试
- 性能基准测试
- 内存泄漏检查(Valgrind)

### Phase 2: Go集成(1-2天)

**2.1 CGo绑定(0.5天)**
- 编写Go binding代码
- 错误处理封装
- 资源管理(finalizer)

**2.2 Pipeline集成(0.5天)**
- 在对话开始前调用识别
- 超时保护(1.5秒)
- 降级策略(失败→访客模式)

**2.3 用户管理工具(0.5天)**
- CLI命令: `pibuddy user register <name>`
- 语音采样辅助脚本
- 用户列表/删除命令

**2.4 集成测试(0.5天)**
- 端到端测试
- 异常场景测试
- 性能测试

### Phase 3: 部署与文档(1天)

**3.1 编译脚本(0.3天)**
```bash
# build.sh
cd voiceprint-rs
cargo build --release --target aarch64-unknown-linux-gnu
cp target/aarch64-unknown-linux-gnu/release/libvoiceprint.so ../lib/

# 验证动态库
file ../lib/libvoiceprint.so
ldd ../lib/libvoiceprint.so
```

**3.2 部署文档(0.4天)**
- 环境要求说明
- 安装步骤
- 用户注册流程
- 故障排查指南

**3.3 性能优化(0.3天)**
- 应用ONNX量化(INT8)
- 模型缓存预热
- 线程池配置

**总计时间: 6-8天**

## Risks / Trade-offs

### 风险 1: ONNX模型算子不兼容
**概率**: 低(Pyannote使用标准算子)
**缓解措施**:
- 优先使用社区验证的ONNX模型
- 预先测试模型加载(Python ONNX Runtime验证)
- 备选方案: 切换到Sherpa-ONNX(支持更多算子)

### 风险 2: Rust库崩溃导致Go主程序退出
**概率**: 中(FFI边界易出错)
**缓解措施**:
- Rust端捕获所有panic: `std::panic::catch_unwind`
- 返回错误码而非panic
- Go端recover机制
- 充分的边界测试

### 风险 3: 交叉编译到ARM64失败
**概率**: 低(工具链成熟)
**缓解措施**:
- 使用官方工具链: `rustup target add aarch64-unknown-linux-gnu`
- Docker容器编译: `rust:1.75-slim`
- 最坏情况: 树莓派上原生编译(需30分钟)

### 风险 4: 识别准确率不达标
**概率**: 低(使用预训练SOTA模型)
**缓解措施**:
- 要求用户注册3-5个样本
- 动态阈值调整(初始0.75)
- 环境噪音过滤(VAD)
- 备选: 更换wespeaker模型

### Trade-off 1: 无法训练自定义模型
**选择**: 使用预训练模型,放弃自定义训练能力
**理由**: 家庭场景数据量不足,训练收益低

### Trade-off 2: Rust开发学习曲线
**选择**: 接受1-2天学习成本,换取长期性能优势
**理由**: PiBuddy核心开发者已熟悉Rust,技术风险可控

## Migration Plan

### 部署步骤

**1. 编译Rust库(开发机)**
```bash
cd pibuddy/voiceprint-rs
./build-for-pi.sh  # 交叉编译到ARM64
```

**2. 部署到树莓派**
```bash
# 复制文件
scp lib/libvoiceprint.so pi@pibuddy:/opt/pibuddy/lib/
scp models/pyannote_embedding.onnx pi@pibuddy:/opt/pibuddy/models/

# 验证依赖
ssh pi@pibuddy "ldd /opt/pibuddy/lib/libvoiceprint.so"
```

**3. 初始化数据库**
```bash
pibuddy user init  # 创建users.db
```

**4. 用户注册**
```bash
# 语音引导注册
pibuddy user register xiaoming
# 系统提示: "请说3-5句话,例如'我是小明,今天天气真好'..."
# 采集完成后自动保存
```

**5. 启动验证**
```bash
pibuddy run
# 唤醒后系统识别: "Hi 小明! 有什么可以帮你?"
```

### 回滚方案
```bash
# 禁用声纹识别
pibuddy config set voiceprint.enabled false

# 或移除动态库(Go自动降级)
rm /opt/pibuddy/lib/libvoiceprint.so
```

### 监控指标
```go
// Go端埋点
metrics.Histogram("voiceprint.identify.latency")     // 识别延迟
metrics.Counter("voiceprint.identify.success")       // 成功次数
metrics.Counter("voiceprint.identify.guest")         // 访客次数
metrics.Gauge("voiceprint.memory.usage")             // 内存占用
```

## Performance Targets

| 指标 | 目标值 | 测试方法 |
|------|--------|----------|
| 识别延迟(P50) | <1秒 | 100次识别取中位数 |
| 识别延迟(P99) | <1.5秒 | 100次识别取99分位 |
| 内存占用(稳态) | <150MB | `ps aux | grep voiceprint` |
| 准确率 | >85% | 5个用户×20次识别 |
| 库文件大小 | <20MB | `du -h libvoiceprint.so` |
| 启动时间 | <100ms | `time voiceprint_init()` |

## Open Questions

1. **用户注册UX优化**: 
   - 当前方案: CLI工具 + 3次语音采样
   - 待优化: 对话式注册("你好PiBuddy,我是新用户")
   - 决策时机: MVP验证后

2. **识别失败重试策略**:
   - 当前方案: 单次失败直接访客模式
   - 待讨论: 是否允许1次重试(用户说"识别错了"时)
   - 决策依据: 用户反馈

3. **多用户同时在场**:
   - 当前方案: 仅识别主要说话人
   - 待讨论: 是否需要"我是xxx"主动声明机制
   - 决策时机: 实际使用场景明确后

4. **模型更新策略**:
   - 当前方案: 手动下载新ONNX模型
   - 待讨论: 是否需要自动检查更新
   - 决策: 暂不实现(家庭设备稳定性优先)

5. **隐私数据备份**:
   - 当前方案: SQLite本地存储
   - 待讨论: 是否提供加密备份到NAS
   - 决策: 后续需求驱动

## Appendix

### 测试音频准备
```bash
# 录制测试音频(3秒)
arecord -D plughw:1,0 -f S16_LE -r 16000 -c 1 -d 3 test_user1.wav

# 批量生成测试集
for i in {1..5}; do
  arecord -D plughw:1,0 -f S16_LE -r 16000 -c 1 -d 3 user1_sample_$i.wav
done
```

### 相关资源链接
- Pyannote ONNX模型: https://huggingface.co/nevil-ramani/pyannote_embedding_onnx
- Tract文档: https://docs.rs/tract-onnx/latest/tract_onnx/
- Sherpa-ONNX(备选方案): https://github.com/k2-fsa/sherpa-onnx
- 声纹识别论文: "Speaker Verification Using Adapted Gaussian Mixture Models" (Reynolds et al.)
