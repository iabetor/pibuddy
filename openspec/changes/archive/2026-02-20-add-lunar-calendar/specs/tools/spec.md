# Tools Capability - Spec Deltas

## ADDED Requirements

### Requirement: Lunar Calendar Query Tool

系统 SHALL 提供 `get_lunar_date` 工具，用于查询农历日期和传统历法信息。

**工具定义：**
- 名称：`get_lunar_date`
- 描述：查询农历日期和传统历法信息。当用户询问农历日期、干支纪年、生肖、节气、传统节日、黄历宜忌等问题时使用。
- 参数：
  - `include_huangli` (boolean, 可选): 是否包含黄历信息（宜忌、冲煞等），默认 false

**返回信息包括：**
1. **基础信息**
   - 公历日期 (solar_date)
   - 农历日期 (lunar_date)
   - 年干支 (year_ganzhi)
   - 生肖 (zodiac)
   - 月干支 (month_ganzhi)
   - 日干支 (day_ganzhi)
   - 星座 (constellation)

2. **节气信息**
   - 当前节气状态 (solar_term)
   - 下一个节气 (next_solar_term)

3. **节日信息**
   - 传统节日 (festivals)
   - 法定节假日（如有）

4. **黄历信息**（可选）
   - 宜 (yi)
   - 忌 (ji)
   - 冲 (chong)
   - 煞 (sha)

#### Scenario: 查询当天农历日期

- **WHEN** 用户询问"今天农历几号"
- **THEN** 系统调用 `get_lunar_date` 工具（参数：`{}`）
- **AND** 返回包含农历日期、干支、生肖的信息
- **AND** LLM 生成自然语言回答："今天是农历二〇二六年腊月廿八，丙寅(虎)年"

#### Scenario: 查询生肖年份

- **WHEN** 用户询问"今年是什么年"
- **THEN** 系统调用 `get_lunar_date` 工具
- **AND** 返回干支纪年和生肖信息
- **AND** LLM 回答："今年是丙寅年，生肖属虎"

#### Scenario: 查询节气信息

- **WHEN** 用户询问"现在是什么节气"或"离立春还有几天"
- **THEN** 系统调用 `get_lunar_date` 工具
- **AND** 返回当前节气状态和下一个节气信息
- **AND** LLM 根据返回信息回答用户问题

#### Scenario: 查询黄历信息

- **WHEN** 用户询问"今天宜做什么"或"今天什么冲什么"
- **THEN** 系统调用 `get_lunar_date` 工具（参数：`{"include_huangli": true}`）
- **AND** 返回包含宜忌、冲煞的黄历信息
- **AND** LLM 生成自然语言回答："今日宜：祭祀、祈福...；忌：动土、破土..."

#### Scenario: 查询节日

- **WHEN** 用户询问"春节是哪天"或"离春节还有几天"
- **THEN** 系统调用 `get_lunar_date` 工具
- **AND** 返回节日信息（包括节日名称和日期）
- **AND** LLM 计算并回答日期差和具体日期

### Requirement: Lunar Calendar Tool Accuracy

农历工具 SHALL 提供准确的农历转换和历法信息。

**准确性要求：**
1. 支持 1900-2100 年的日期转换
2. 干支计算符合传统历法规则
3. 生肖与年份对应准确
4. 节气时间误差不超过 1 分钟
5. 传统节日日期准确

#### Scenario: 验证干支纪年

- **WHEN** 查询 2026 年的干支纪年
- **THEN** 系统返回"丙寅年"
- **AND** 生肖返回"虎"

#### Scenario: 验证节气计算

- **WHEN** 查询 2026 年 2 月 4 日附近的节气
- **THEN** 系统正确识别"立春"节气（2026年2月4日）

#### Scenario: 验证传统节日

- **WHEN** 查询 2026 年春节日期
- **THEN** 系统返回正确的春节日期（农历正月初一对应的公历日期）

### Requirement: Lunar Calendar Tool Performance

农历工具 SHALL 快速响应查询请求。

**性能要求：**
1. 查询响应时间 < 10ms（本地计算）
2. 无外部 API 调用
3. 低内存占用（< 1MB）

#### Scenario: 快速响应查询

- **WHEN** 用户查询农历信息
- **THEN** 工具在 10ms 内返回结果
- **AND** 不发起网络请求

### Requirement: Lunar Calendar Tool Error Handling

农历工具 SHALL 正确处理错误情况。

**错误处理：**
1. 日期超出支持范围（1900-2100）时返回明确错误信息
2. 参数解析失败时返回友好错误提示
3. 内部计算错误时记录日志并返回错误

#### Scenario: 日期超出范围

- **WHEN** 系统内部日期异常（超出 1900-2100 年）
- **THEN** 工具返回错误信息："日期超出支持范围"
- **AND** 不导致系统崩溃

#### Scenario: 参数解析失败

- **WHEN** 工具收到无效的 JSON 参数
- **THEN** 工具返回错误信息："参数解析失败"
- **AND** 使用默认参数继续执行
