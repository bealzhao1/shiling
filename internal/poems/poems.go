// Package poems 提供内置诗词库，供飞花令 Agent 检索。
package poems

import "strings"

// Poem 一首诗的数据结构。
type Poem struct {
	Text   string // 诗句，如 "床前明月光，疑是地上霜"
	Author string // 作者
	Title  string // 诗名
}

// all 内置诗词库。每首诗标注了所含的关键字（令字），用于飞花令检索。
var all = []Poem{
	// —— 月 ——
	{Text: "床前明月光，疑是地上霜", Author: "李白", Title: "静夜思"},
	{Text: "举头望明月，低头思故乡", Author: "李白", Title: "静夜思"},
	{Text: "海上生明月，天涯共此时", Author: "张九龄", Title: "望月怀远"},
	{Text: "月落乌啼霜满天，江枫渔火对愁眠", Author: "张继", Title: "枫桥夜泊"},
	{Text: "明月几时有？把酒问青天", Author: "苏轼", Title: "水调歌头"},
	{Text: "春风又绿江南岸，明月何时照我还", Author: "王安石", Title: "泊船瓜洲"},
	{Text: "秦时明月汉时关，万里长征人未还", Author: "王昌龄", Title: "出塞"},
	{Text: "春江潮水连海平，海上明月共潮生", Author: "张若虚", Title: "春江花月夜"},
	{Text: "露从今夜白，月是故乡明", Author: "杜甫", Title: "月夜忆舍弟"},
	{Text: "今夜月明人尽望，不知秋思落谁家", Author: "王建", Title: "十五夜望月"},

	// —— 花 ——
	{Text: "夜来风雨声，花落知多少", Author: "孟浩然", Title: "春晓"},
	{Text: "感时花溅泪，恨别鸟惊心", Author: "杜甫", Title: "春望"},
	{Text: "花间一壶酒，独酌无相亲", Author: "李白", Title: "月下独酌"},
	{Text: "人间四月芳菲尽，山寺桃花始盛开", Author: "白居易", Title: "大林寺桃花"},
	{Text: "接天莲叶无穷碧，映日荷花别样红", Author: "杨万里", Title: "晓出净慈寺送林子方"},
	{Text: "黄四娘家花满蹊，千朵万朵压枝低", Author: "杜甫", Title: "江畔独步寻花"},
	{Text: "不是花中偏爱菊，此花开尽更无花", Author: "元稹", Title: "菊花"},
	{Text: "竹外桃花三两枝，春江水暖鸭先知", Author: "苏轼", Title: "惠崇春江晚景"},
	{Text: "桃花潭水深千尺，不及汪伦送我情", Author: "李白", Title: "赠汪伦"},

	// —— 风 ——
	{Text: "野火烧不尽，春风吹又生", Author: "白居易", Title: "赋得古原草送别"},
	{Text: "随风潜入夜，润物细无声", Author: "杜甫", Title: "春夜喜雨"},
	{Text: "羌笛何须怨杨柳，春风不度玉门关", Author: "王之涣", Title: "凉州词"},
	{Text: "大风起兮云飞扬，威加海内兮归故乡", Author: "刘邦", Title: "大风歌"},
	{Text: "古道西风瘦马，夕阳西下，断肠人在天涯", Author: "马致远", Title: "天净沙·秋思"},
	{Text: "昨夜星辰昨夜风，画楼西畔桂堂东", Author: "李商隐", Title: "无题"},
	{Text: "北风卷地白草折，胡天八月即飞雪", Author: "岑参", Title: "白雪歌送武判官归京"},
	{Text: "柴门闻犬吠，风雪夜归人", Author: "刘长卿", Title: "逢雪宿芙蓉山主人"},

	// —— 春 ——
	{Text: "春眠不觉晓，处处闻啼鸟", Author: "孟浩然", Title: "春晓"},
	{Text: "好雨知时节，当春乃发生", Author: "杜甫", Title: "春夜喜雨"},
	{Text: "春色满园关不住，一枝红杏出墙来", Author: "叶绍翁", Title: "游园不值"},
	{Text: "春风得意马蹄疾，一日看尽长安花", Author: "孟郊", Title: "登科后"},
	{Text: "等闲识得东风面，万紫千红总是春", Author: "朱熹", Title: "春日"},
	{Text: "不知细叶谁裁出，二月春风似剪刀", Author: "贺知章", Title: "咏柳"},
	{Text: "春种一粒粟，秋收万颗子", Author: "李绅", Title: "悯农"},
	{Text: "日出江花红胜火，春来江水绿如蓝", Author: "白居易", Title: "忆江南"},
	{Text: "问君能有几多愁？恰似一江春水向东流", Author: "李煜", Title: "虞美人"},
	{Text: "春宵一刻值千金，花有清香月有阴", Author: "苏轼", Title: "春宵"},

	// —— 山 ——
	{Text: "会当凌绝顶，一览众山小", Author: "杜甫", Title: "望岳"},
	{Text: "空山新雨后，天气晚来秋", Author: "王维", Title: "山居秋暝"},
	{Text: "两岸青山相对出，孤帆一片日边来", Author: "李白", Title: "望天门山"},
	{Text: "山重水复疑无路，柳暗花明又一村", Author: "陆游", Title: "游山西村"},
	{Text: "采菊东篱下，悠然见南山", Author: "陶渊明", Title: "饮酒"},
	{Text: "千山鸟飞绝，万径人踪灭", Author: "柳宗元", Title: "江雪"},
	{Text: "白日依山尽，黄河入海流", Author: "王之涣", Title: "登鹳雀楼"},
	{Text: "只在此山中，云深不知处", Author: "贾岛", Title: "寻隐者不遇"},
	{Text: "黄河远上白云间，一片孤城万仞山", Author: "王之涣", Title: "凉州词"},

	// —— 水 ——
	{Text: "水光潋滟晴方好，山色空蒙雨亦奇", Author: "苏轼", Title: "饮湖上初晴后雨"},
	{Text: "桃花潭水深千尺，不及汪伦送我情", Author: "李白", Title: "赠汪伦"},
	{Text: "抽刀断水水更流，举杯消愁愁更愁", Author: "李白", Title: "宣州谢朓楼饯别校书叔云"},
	{Text: "飞流直下三千尺，疑是银河落九天", Author: "李白", Title: "望庐山瀑布"},
	{Text: "竹外桃花三两枝，春江水暖鸭先知", Author: "苏轼", Title: "惠崇春江晚景"},
	{Text: "问君能有几多愁？恰似一江春水向东流", Author: "李煜", Title: "虞美人"},
	{Text: "行到水穷处，坐看云起时", Author: "王维", Title: "终南别业"},
	{Text: "山重水复疑无路，柳暗花明又一村", Author: "陆游", Title: "游山西村"},

	// —— 云 ——
	{Text: "黄河远上白云间，一片孤城万仞山", Author: "王之涣", Title: "凉州词"},
	{Text: "朝辞白帝彩云间，千里江陵一日还", Author: "李白", Title: "早发白帝城"},
	{Text: "行到水穷处，坐看云起时", Author: "王维", Title: "终南别业"},
	{Text: "众鸟高飞尽，孤云独去闲", Author: "李白", Title: "独坐敬亭山"},
	{Text: "只在此山中，云深不知处", Author: "贾岛", Title: "寻隐者不遇"},
	{Text: "黑云翻墨未遮山，白雨跳珠乱入船", Author: "苏轼", Title: "六月二十七日望湖楼醉书"},
	{Text: "大风起兮云飞扬，威加海内兮归故乡", Author: "刘邦", Title: "大风歌"},
	{Text: "不畏浮云遮望眼，自缘身在最高层", Author: "王安石", Title: "登飞来峰"},

	// —— 雪 ——
	{Text: "窗含西岭千秋雪，门泊东吴万里船", Author: "杜甫", Title: "绝句"},
	{Text: "孤舟蓑笠翁，独钓寒江雪", Author: "柳宗元", Title: "江雪"},
	{Text: "北风卷地白草折，胡天八月即飞雪", Author: "岑参", Title: "白雪歌送武判官归京"},
	{Text: "遥知不是雪，为有暗香来", Author: "王安石", Title: "梅花"},
	{Text: "柴门闻犬吠，风雪夜归人", Author: "刘长卿", Title: "逢雪宿芙蓉山主人"},
	{Text: "欲渡黄河冰塞川，将登太行雪满山", Author: "李白", Title: "行路难"},

	// —— 日 ——
	{Text: "日照香炉生紫烟，遥看瀑布挂前川", Author: "李白", Title: "望庐山瀑布"},
	{Text: "白日依山尽，黄河入海流", Author: "王之涣", Title: "登鹳雀楼"},
	{Text: "锄禾日当午，汗滴禾下土", Author: "李绅", Title: "悯农"},
	{Text: "迟日江山丽，春风花草香", Author: "杜甫", Title: "绝句"},
	{Text: "大漠孤烟直，长河落日圆", Author: "王维", Title: "使至塞上"},
	{Text: "海日生残夜，江春入旧年", Author: "王湾", Title: "次北固山下"},

	// —— 人 ——
	{Text: "独在异乡为异客，每逢佳节倍思亲", Author: "王维", Title: "九月九日忆山东兄弟"},
	{Text: "故人西辞黄鹤楼，烟花三月下扬州", Author: "李白", Title: "黄鹤楼送孟浩然之广陵"},
	{Text: "劝君更尽一杯酒，西出阳关无故人", Author: "王维", Title: "送元二使安西"},
	{Text: "莫愁前路无知己，天下谁人不识君", Author: "高适", Title: "别董大"},
	{Text: "遥知兄弟登高处，遍插茱萸少一人", Author: "王维", Title: "九月九日忆山东兄弟"},
	{Text: "空山不见人，但闻人语响", Author: "王维", Title: "鹿柴"},

	// —— 夜 ——
	{Text: "随风潜入夜，润物细无声", Author: "杜甫", Title: "春夜喜雨"},
	{Text: "月落乌啼霜满天，江枫渔火对愁眠", Author: "张继", Title: "枫桥夜泊"},
	{Text: "姑苏城外寒山寺，夜半钟声到客船", Author: "张继", Title: "枫桥夜泊"},
	{Text: "海日生残夜，江春入旧年", Author: "王湾", Title: "次北固山下"},
	{Text: "柴门闻犬吠，风雪夜归人", Author: "刘长卿", Title: "逢雪宿芙蓉山主人"},
	{Text: "昨夜星辰昨夜风，画楼西畔桂堂东", Author: "李商隐", Title: "无题"},
}

// Search 按关键字（令字/主题词）检索诗句。匹配诗句文本或标题。
func Search(keyword string) []Poem {
	if strings.TrimSpace(keyword) == "" {
		return nil
	}
	var out []Poem
	for _, p := range all {
		if strings.Contains(p.Text, keyword) || strings.Contains(p.Title, keyword) || strings.Contains(p.Author, keyword) {
			out = append(out, p)
		}
	}
	return out
}

// All 返回全部诗词。
func All() []Poem {
	return all
}
