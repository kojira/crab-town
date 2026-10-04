package world

// The town: one 58x20 room holding two houses and a garden between them.
//
//	x=0..21    らぼみの家 (labomi-house, 22x20, front door on its right wall)
//	x=22..25   garden (public, stepping stones at y=4 join the two front doors)
//	x=26..57   のすたろうの家 (nostarou-house, the 4LDK of layout.go moved by +26)
const (
	TownID      = "town"
	TownWidth   = 58
	TownHeight  = 20
	LabomiX     = 0
	LabomiW     = 22
	GardenX     = 22
	GardenW     = 4
	NostarouX   = 26
	LabomiHouse = "labomi-house"
	// NostarouHouse is also where a legacy CRAB_INVITED ("id,id") applies.
	NostarouHouse = "nostarou-house"
)

// Kinds used by the town / labomi's house (dot art in web/town.js).
const (
	KindPinkSofa  = "pinksofa"
	KindPinkBed   = "pinkbed"
	KindDresser   = "dresser"
	KindHeartRug  = "heartrug"
	KindPlush     = "plush"
	KindRingLight = "ringlight"
	KindFuton     = "futon"
	KindLaptop    = "laptop"
	KindStone     = "steppingstone"
	KindTree      = "tree"
	KindFlower    = "flower"
	KindPostbox   = "postbox"
)

// らぼみの家 (absolute town coordinates, x=0..21):
//
//	y=0        outer wall (window in the LDK); front door (21,4) to the garden
//	y=1..8     LDK (x=1..15) | entrance (x=17..20)
//	y=9        wall, doors from the LDK and the entrance to the hallway
//	y=10..11   hallway
//	y=12       wall, one door per room
//	y=13..18   自室 | おとまり部屋 | toilet / washroom (y=13..15), bath (y=17..18)
func labomiZones() []*Zone {
	z := func(id, name, floor, vis string, r Rect, private bool) *Zone {
		return &Zone{ID: "labomi-" + id, Name: name, Floor: floor, Visibility: vis, Rect: r, Private: private, House: LabomiHouse}
	}
	return []*Zone{
		z("ldk", "LDK", "pinkwood", VisPublic, Rect{1, 1, 15, 8}, false),
		z("entrance", "玄関", "marble", VisPublic, Rect{17, 1, 4, 8}, false),
		z("hallway", "廊下", "hall", VisPublic, Rect{1, 10, 20, 2}, false),
		z("room", "らぼみの部屋", "pinkcarpet", VisOwner, Rect{1, 13, 7, 6}, false),
		z("guest", "おとまり部屋", "mat", VisInvited, Rect{9, 13, 5, 6}, false),
		z("toilet", "トイレ", "bathtile", VisOwner, Rect{15, 13, 2, 3}, true),
		z("washroom", "洗面所", "cushionfloor", VisOwner, Rect{18, 13, 3, 3}, false),
		z("bath", "浴室", "bathtile", VisOwner, Rect{15, 17, 6, 2}, true),
	}
}

func labomiWalls() []Rect {
	return []Rect{
		{0, 0, LabomiW, 1}, {0, TownHeight - 1, LabomiW, 1}, // outer top / bottom
		{0, 0, 1, TownHeight}, {LabomiW - 1, 0, 1, TownHeight}, // outer left / right
		{16, 1, 1, 8},                 // LDK | entrance
		{0, 9, LabomiW, 1},            // upper floor | hallway
		{0, 12, LabomiW, 1},           // hallway | rooms
		{8, 13, 1, 6}, {14, 13, 1, 6}, // 自室 | おとまり | water area
		{17, 13, 1, 3}, {15, 16, 6, 1}, // toilet | washroom, both | bath
	}
}

func labomiDoors() []Pos {
	return []Pos{
		{21, 4},         // front door (garden)
		{16, 3},         // entrance -> LDK
		{8, 9}, {18, 9}, // LDK / entrance -> hallway
		{4, 12}, {11, 12}, {16, 12}, {19, 12}, // hallway -> 自室 / おとまり / toilet / washroom
		{19, 16}, // washroom -> bath
	}
}

func labomiFurniture() []*Furniture {
	deco := func(id, kind, label string, x, y, w, h int) *Furniture {
		return &Furniture{ID: "labomi-" + id, Kind: kind, Label: label, Pos: Pos{x, y}, Size: Size{w, h}}
	}
	flat := func(id, kind, label string, x, y, w, h int) *Furniture {
		f := deco(id, kind, label, x, y, w, h)
		f.Walkable = true
		return f
	}
	use := func(id, kind, label, fn, state string, x, y, w, h int, access Pos) *Furniture {
		f := deco(id, kind, label, x, y, w, h)
		f.Function, f.State, f.Access = fn, state, access
		return f
	}
	return []*Furniture{
		// LDK: kitchen corner, dining, living
		deco("counter", KindCounter, "キッチン台", 1, 1, 3, 1),
		deco("stove", KindStove, "コンロ", 4, 1, 2, 1),
		deco("fridge", KindFridge, "冷蔵庫", 6, 1, 1, 2),
		flat("kitchenmat", KindKitchenMat, "キッチンマット", 1, 2, 3, 1),
		deco("table", KindTable, "ダイニングテーブル", 3, 4, 2, 2),
		deco("chair1", KindChair, "椅子", 2, 4, 1, 1),
		deco("chair2", KindChair, "椅子", 5, 5, 1, 1),
		use("window", KindWindow, "窓", "timeline", StateTalking, 10, 0, 4, 1, Pos{11, 1}),
		flat("rug", KindHeartRug, "ハートのラグ", 10, 2, 4, 2),
		use("sofa", KindPinkSofa, "ピンクのソファ", "visitors", StateTalking, 10, 5, 4, 2, Pos{11, 4}),
		deco("ringlight", KindRingLight, "自撮りライト", 15, 1, 1, 1),
		deco("plush", KindPlush, "ぬいぐるみ", 15, 8, 1, 1),
		deco("plant", KindPlant, "観葉植物", 1, 8, 1, 1),
		deco("trash", KindTrash, "ゴミ箱", 7, 8, 1, 1),
		// entrance
		deco("shoebox", KindShoebox, "下駄箱", 20, 1, 1, 3),
		flat("doormat", KindDoormat, "玄関マット", 20, 4, 1, 1),
		deco("umbrella", KindUmbrella, "傘立て", 17, 1, 1, 1),
		deco("entplant", KindPlant, "観葉植物", 20, 8, 1, 1),
		// hallway
		deco("hallplant", KindPlant, "観葉植物", 1, 10, 1, 1),
		// 自室
		use("pc", KindLaptop, "ノートPC", "work-container", StateWorking, 1, 13, 2, 1, Pos{1, 14}),
		use("bed", KindPinkBed, "ベッド", "standby", StateAway, 1, 15, 2, 3, Pos{3, 16}),
		deco("dresser", KindDresser, "ドレッサー", 6, 13, 2, 1),
		flat("heartrug", KindHeartRug, "ハートのラグ", 4, 16, 3, 2),
		deco("plush2", KindPlush, "ぬいぐるみ", 7, 18, 1, 1),
		// おとまり部屋
		deco("futon1", KindFuton, "布団", 9, 15, 2, 3),
		deco("futon2", KindFuton, "布団", 12, 15, 2, 3),
		deco("guestlamp", KindFloorLamp, "フロアランプ", 9, 13, 1, 1),
		// toilet
		deco("toiletbowl", KindToilet, "便器", 15, 13, 1, 1),
		deco("paper", KindPaper, "トイレットペーパー", 15, 15, 1, 1),
		flat("toiletmat", KindToiletMat, "トイレマット", 16, 14, 1, 1),
		// washroom
		deco("washer", KindWasher, "洗濯機", 18, 13, 1, 1),
		deco("vanity", KindVanity, "洗面台", 20, 13, 1, 1),
		deco("laundry", KindLaundry, "洗濯カゴ", 18, 15, 1, 1),
		flat("bathmat", KindBathMat, "バスマット", 20, 15, 1, 1),
		// bath
		deco("bathtub", KindBathtub, "浴槽", 15, 17, 2, 2),
		deco("shower", KindShower, "シャワー", 20, 17, 1, 1),
	}
}

// garden: public, outside both houses.
func gardenZone() *Zone {
	return &Zone{ID: "garden", Name: "庭", Floor: "grass", Visibility: VisPublic, Rect: Rect{GardenX, 0, GardenW, TownHeight}}
}

func gardenFurniture() []*Furniture {
	out := []*Furniture{}
	for x := GardenX; x < GardenX+GardenW; x++ { // stepping stones: labomi door -> nostarou door
		out = append(out, &Furniture{ID: "stone" + string(rune('1'+x-GardenX)), Kind: KindStone, Label: "飛び石", Pos: Pos{x, 4}, Size: Size{1, 1}, Walkable: true})
	}
	deco := func(id, kind, label string, x, y int) *Furniture {
		return &Furniture{ID: id, Kind: kind, Label: label, Pos: Pos{x, y}, Size: Size{1, 1}}
	}
	return append(out,
		deco("postbox-labomi", KindPostbox, "らぼみの郵便受け", 22, 2),
		deco("postbox-nostarou", KindPostbox, "のすたろうの郵便受け", 25, 6),
		deco("tree1", KindTree, "木", 24, 0),
		deco("tree2", KindTree, "木", 23, 11),
		deco("tree3", KindTree, "木", 25, 17),
		deco("flower1", KindFlower, "花壇", 22, 8),
		deco("flower2", KindFlower, "花壇", 25, 9),
		deco("flower3", KindFlower, "花壇", 22, 15),
	)
}

// NewDefault builds the town: labomi's house, the garden, nostarou's house,
// and the two residents at home. Each owner invites nobody until
// SetInvited (CRAB_INVITED) says so.
func NewDefault() *World {
	w := New()
	r := &Room{
		ID: TownID, Visibility: VisPublic, Width: TownWidth, Height: TownHeight,
		Houses: []*House{
			{ID: LabomiHouse, Owner: "labomi", Invited: []string{}, Rect: Rect{LabomiX, 0, LabomiW, TownHeight}},
			{ID: NostarouHouse, Owner: "nostarou", Invited: []string{}, Rect: Rect{NostarouX, 0, HouseWidth, HouseHeight}},
		},
	}
	r.Zones = append(labomiZones(), gardenZone())
	r.Zones = append(r.Zones, shiftZones(defaultZones(), NostarouX, NostarouHouse)...)
	r.Walls = append(labomiWalls(), shiftRects(defaultWalls(), NostarouX)...)
	r.Doors = append(labomiDoors(), shiftPos(defaultDoors(), NostarouX)...)
	r.Furniture = append(labomiFurniture(), gardenFurniture()...)
	r.Furniture = append(r.Furniture, shiftFurniture(defaultFurniture(), NostarouX)...)
	w.AddRoom(r)
	w.AddActor(&Actor{ID: "labomi", Name: "らぼみ", RoomID: TownID, Pos: Pos{8, 6}, State: StateIdle})
	w.AddActor(&Actor{ID: "nostarou", Name: "のすたろう", RoomID: TownID, Pos: Pos{NostarouX + 22, 3}, State: StateIdle})
	return w
}
