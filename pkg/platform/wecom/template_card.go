package wecom

// TemplateCard 模版卡片结构体，涵盖多种卡片类型。
// 对应文档：5_模版卡片类型.md
type TemplateCard struct {
	CardType              string              `json:"card_type"`                         // 模版类型: text_notice, news_notice, button_interaction, vote_interaction, multiple_interaction
	Source                *Source             `json:"source,omitempty"`                  // 卡片来源样式信息
	ActionMenu            *ActionMenu         `json:"action_menu,omitempty"`             // 卡片右上角更多操作按钮
	MainTitle             *MainTitle          `json:"main_title,omitempty"`              // 一级标题
	EmphasisContent       *EmphasisContent    `json:"emphasis_content,omitempty"`        // 关键数据样式
	QuoteArea             *QuoteArea          `json:"quote_area,omitempty"`              // 引用文献样式
	SubTitleText          string              `json:"sub_title_text,omitempty"`          // 二级普通文本
	VerticalContentList   []VerticalContent   `json:"vertical_content_list,omitempty"`   // 二级垂直内容列表
	HorizontalContentList []HorizontalContent `json:"horizontal_content_list,omitempty"` // 二级标题+文本列表
	JumpList              []JumpAction        `json:"jump_list,omitempty"`               // 跳转指引样式列表
	CardAction            *CardAction         `json:"card_action,omitempty"`             // 整体卡片点击跳转
	TaskID                string              `json:"task_id,omitempty"`                 // 任务id
	CardImage             *CardImage          `json:"card_image,omitempty"`              // 图片样式 (news_notice)
	ImageTextArea         *ImageTextArea      `json:"image_text_area,omitempty"`         // 左图右文样式 (news_notice)
	ButtonSelection       *SelectionItem      `json:"button_selection,omitempty"`        // 下拉式选择器 (button_interaction)
	ButtonList            []Button            `json:"button_list,omitempty"`             // 按钮列表 (button_interaction)
	Checkbox              *Checkbox           `json:"checkbox,omitempty"`                // 选择题样式 (vote_interaction)
	SubmitButton          *SubmitButton       `json:"submit_button,omitempty"`           // 提交按钮 (vote_interaction, multiple_interaction)
	SelectList            []SelectionItem     `json:"select_list,omitempty"`             // 下拉式选择器列表 (multiple_interaction)
	Feedback              *FeedbackInfo       `json:"feedback,omitempty"`                // 反馈信息 (主动回复时使用)
}

// Source 卡片来源样式信息
type Source struct {
	IconURL   string `json:"icon_url,omitempty"`   // 来源图片的url
	Desc      string `json:"desc,omitempty"`       // 来源图片的描述
	DescColor int    `json:"desc_color,omitempty"` // 来源文字的颜色: 0(默认)灰色, 1黑色, 2红色, 3绿色
}

// ActionMenu 卡片右上角更多操作按钮
type ActionMenu struct {
	Desc       string       `json:"desc"`        // 更多操作界面的描述
	ActionList []ActionItem `json:"action_list"` // 操作列表 [1, 3]
}

// ActionItem 操作列表项
type ActionItem struct {
	Text string `json:"text"` // 操作的描述文案
	Key  string `json:"key"`  // 操作key值
}

// MainTitle 模版卡片的主要内容
type MainTitle struct {
	Title string `json:"title,omitempty"` // 一级标题
	Desc  string `json:"desc,omitempty"`  // 标题辅助信息
}

// EmphasisContent 关键数据样式
type EmphasisContent struct {
	Title string `json:"title,omitempty"` // 关键数据内容
	Desc  string `json:"desc,omitempty"`  // 关键数据描述
}

// QuoteArea 引用文献样式
type QuoteArea struct {
	Type      int    `json:"type,omitempty"`       // 点击事件: 0无, 1跳转url, 2跳转小程序
	URL       string `json:"url,omitempty"`        // 跳转url
	AppID     string `json:"appid,omitempty"`      // 小程序appid
	PagePath  string `json:"pagepath,omitempty"`   // 小程序pagepath
	Title     string `json:"title,omitempty"`      // 标题
	QuoteText string `json:"quote_text,omitempty"` // 引用文案
}

// HorizontalContent 二级标题+文本列表
type HorizontalContent struct {
	Type    int    `json:"type,omitempty"`   // 链接类型: 0普通文本, 1跳转url, 3点击跳转成员详情
	KeyName string `json:"keyname"`          // 二级标题
	Value   string `json:"value,omitempty"`  // 二级文本
	URL     string `json:"url,omitempty"`    // 跳转url
	UserID  string `json:"userid,omitempty"` // 成员详情userid
}

// VerticalContent 卡片二级垂直内容
type VerticalContent struct {
	Title string `json:"title"`          // 二级标题
	Desc  string `json:"desc,omitempty"` // 二级普通文本
}

// JumpAction 跳转指引样式的列表
type JumpAction struct {
	Type     int    `json:"type,omitempty"`     // 跳转类型: 0无, 1url, 2小程序, 3触发消息智能回复
	Question string `json:"question,omitempty"` // 智能问答问题 (type=3)
	Title    string `json:"title"`              // 文案内容
	URL      string `json:"url,omitempty"`      // 跳转url
	AppID    string `json:"appid,omitempty"`    // 小程序appid
	PagePath string `json:"pagepath,omitempty"` // 小程序pagepath
}

// CardAction 整体卡片的点击跳转事件
type CardAction struct {
	Type     int    `json:"type"`               // 跳转类型: 1url, 2小程序
	URL      string `json:"url,omitempty"`      // 跳转url
	AppID    string `json:"appid,omitempty"`    // 小程序appid
	PagePath string `json:"pagepath,omitempty"` // 小程序pagepath
}

// CardImage 图片样式
type CardImage struct {
	URL         string  `json:"url"`                    // 图片url
	AspectRatio float64 `json:"aspect_ratio,omitempty"` // 宽高比 1.3 ~ 2.25
}

// ImageTextArea 左图右文样式
type ImageTextArea struct {
	Type     int    `json:"type,omitempty"`     // 点击事件: 0无, 1url, 2小程序
	URL      string `json:"url,omitempty"`      // 跳转url
	AppID    string `json:"appid,omitempty"`    // 小程序appid
	PagePath string `json:"pagepath,omitempty"` // 小程序pagepath
	Title    string `json:"title,omitempty"`    // 标题
	Desc     string `json:"desc,omitempty"`     // 描述
	ImageURL string `json:"image_url"`          // 图片url
}

// Button 按钮列表
type Button struct {
	Text  string `json:"text"`            // 按钮文案
	Style int    `json:"style,omitempty"` // 按钮样式 1~4
	Key   string `json:"key"`             // 按钮key
}

// SelectionItem 下拉式的选择器
type SelectionItem struct {
	QuestionKey string         `json:"question_key"`          // 题目key
	Title       string         `json:"title,omitempty"`       // 标题
	Disable     bool           `json:"disable,omitempty"`     // 是否不可选 (更新时有效)
	SelectedID  string         `json:"selected_id,omitempty"` // 默认选中id
	OptionList  []SelectOption `json:"option_list"`           // 选项列表
}

// SelectOption 选择器选项
type SelectOption struct {
	ID   string `json:"id"`   // 选项id
	Text string `json:"text"` // 选项文案
}

// Checkbox 选择题样式
type Checkbox struct {
	QuestionKey string           `json:"question_key"`      // 题目key
	Disable     bool             `json:"disable,omitempty"` // 是否不可选 (更新时有效)
	Mode        int              `json:"mode,omitempty"`    // 模式: 0单选, 1多选
	OptionList  []CheckboxOption `json:"option_list"`       // 选项列表
}

// CheckboxOption 选择题选项
type CheckboxOption struct {
	ID        string `json:"id"`         // 选项id
	Text      string `json:"text"`       // 选项文案
	IsChecked bool   `json:"is_checked"` // 是否默认选中
}

// SubmitButton 提交按钮样式
type SubmitButton struct {
	Text string `json:"text"` // 按钮文案
	Key  string `json:"key"`  // 按钮key
}

// FeedbackInfo 反馈信息 (用于主动回复)
type FeedbackInfo struct {
	ID string `json:"id,omitempty"` // 反馈ID
}
