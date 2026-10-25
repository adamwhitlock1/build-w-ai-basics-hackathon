package main

import (
	"fmt"
	"time"

	"kinlight/internal/layout"
	"kinlight/internal/llm"
)

// The demo families, as data. To add a member, add a demoMember; to give
// members something new (chores, say), add a field here and a step in
// seedDemo.
//
// The items are hand-written stand-ins for search results. Their links go to
// real, stable home pages rather than made-up article addresses, so nothing
// a tester clicks is a 404. Refreshing a card replaces them with real ones.

// defaultDemoDB is where make demo writes. Its name ends in ".demo", which is
// what lets the seed delete and rebuild it (see resetDemoDB).
const defaultDemoDB = "porchlight.db.demo"

// demoFamily is one family: its creator is members[0], who must be a parent.
type demoFamily struct {
	name    string
	members []demoMember
	// invites are people invited who haven't joined yet.
	invites []demoInvite
}

type demoMember struct {
	email     string
	parent    bool
	role      string // what this account is for, printed by the seed
	interests string
	location  string
	// calendar is the fixture feed this member's calendar link points to
	// (see calendar.FixtureLink), or "" for no calendar.
	calendar string
	widgets  []demoWidget
	// layout arranges the member's page, given the ids their widgets were
	// saved with (in the order of widgets). nil keeps the default page.
	layout func(widgetIDs []int64) layout.Data
	// articles and events are remembered as their Articles and "Happening
	// near you" results. They're functions of the seed time so that dates
	// in them are near it.
	articles func(now time.Time) []llm.Item
	events   func(now time.Time) []llm.Item
	// age sets a kid's birth year (this year minus age), which picks the
	// chat persona. 0 leaves it unset.
	age int
	// chats are a kid's sample conversations with the family agent.
	chats []demoChat
}

// demoChat is a sample conversation: turns alternate between the kid and
// the agent, starting with the kid.
type demoChat struct {
	mode, title string
	turns       []string
	// flag, if set, flags one of the kid's turns for parents to review.
	flag *demoFlag
}

type demoFlag struct {
	turn                       int // index into turns: a kid's turn
	category, severity, reason string
}

type demoWidget struct {
	title, source, query, prompt string
	archetype                    llm.Archetype
	items                        func(now time.Time) []llm.Item
}

type demoInvite struct {
	email  string
	parent bool
	role   string
}

// day writes the date n days after now the way search results usually do:
// "Sat, Oct 17".
func day(now time.Time, n int) string {
	return now.AddDate(0, 0, n).Format("Mon, Jan 2")
}

var demoFamilies = []demoFamily{
	{
		name: "Rivera Family",
		members: []demoMember{
			{
				email:     "maya@rivera.test",
				parent:    true,
				role:      "parent, created the family; three widgets and a custom page",
				interests: "space launches, vegetable gardening, Denver city news, easy weeknight dinners",
				location:  "Denver, CO",
				calendar:  "maya",
				widgets: []demoWidget{
					{
						title: "Denver Weather", source: "weather_forecast", query: "Denver, CO",
						prompt: "weather in Denver this week", archetype: llm.ArchetypeTable,
						items: denverWeek,
					},
					{
						title: "Upcoming Rocket Launches", source: "news_search", query: "upcoming rocket launches 2026",
						prompt: "show me upcoming rocket launches", archetype: llm.ArchetypeList,
						items: func(now time.Time) []llm.Item {
							return []llm.Item{
								{Title: "Starlink group launch", URL: "https://www.spacex.com/launches/", Snippet: "A Falcon 9 carries another batch of Starlink satellites from Cape Canaveral.", Date: day(now, 2) + ", 6:40 PM ET", Label: "SpaceX"},
								{Title: "Artemis program update", URL: "https://www.nasa.gov/humans-in-space/artemis/", Snippet: "NASA shares the latest on crew training and the next Moon mission.", Date: day(now, 5), Label: "NASA"},
								{Title: "Rocket Lab Electron launch", URL: "https://www.rocketlabusa.com/next-mission/", Snippet: "A small-satellite launch from New Zealand's Mahia Peninsula.", Date: day(now, 9), Label: "Rocket Lab"},
							}
						},
					},
					{
						title: "Weeknight Dinner Ideas", source: "web_search", query: "easy weeknight dinner recipes",
						prompt: "easy weeknight dinner ideas", archetype: llm.ArchetypeCards,
						items: func(time.Time) []llm.Item {
							return []llm.Item{
								{Title: "Budget Bytes", URL: "https://www.budgetbytes.com/", Snippet: "Cheap, quick recipes with cost-per-serving for each one.", Label: "budgetbytes.com"},
								{Title: "NYT Cooking: Easy Weeknight", URL: "https://cooking.nytimes.com/", Snippet: "Sheet-pan dinners and 30-minute pastas.", Label: "cooking.nytimes.com"},
								{Title: "Serious Eats", URL: "https://www.seriouseats.com/", Snippet: "Tested techniques for fast stir-fries and one-pot meals.", Label: "seriouseats.com"},
							}
						},
					},
				},
				layout: mayaLayout,
				articles: func(now time.Time) []llm.Item {
					return []llm.Item{
						{Title: "What's launching this week", URL: "https://spaceflightnow.com/launch-schedule/", Snippet: "A busy week at the Cape, with two Falcon 9 flights scheduled.", Date: day(now, 0), Label: "Spaceflight Now"},
						{Title: "Putting the vegetable garden to bed for winter", URL: "https://extension.colostate.edu/", Snippet: "When to pull the last tomatoes and how to mulch beds before the first hard freeze.", Date: day(now, -1), Label: "CSU Extension"},
						{Title: "City council takes up new bike-lane plan", URL: "https://denverite.com/", Snippet: "The plan would add protected lanes on three east-west streets.", Date: day(now, -1), Label: "Denverite"},
						{Title: "Five 30-minute dinners for busy weeks", URL: "https://www.budgetbytes.com/", Snippet: "One-pan meals that come together faster than takeout.", Date: day(now, -2), Label: "Budget Bytes"},
					}
				},
				events: func(now time.Time) []llm.Item {
					return []llm.Item{
						{Title: "Denver Botanic Gardens fall plant sale", URL: "https://www.botanicgardens.org/", Snippet: "Perennials, bulbs and expert advice.", Date: day(now, 3), Label: "Denver Botanic Gardens"},
						{Title: "Night sky viewing at Chamberlin Observatory", URL: "https://www.du.edu/chamberlin-observatory", Snippet: "Public telescope night, weather permitting.", Date: day(now, 1) + ", 7:30 PM", Label: "Chamberlin Observatory"},
						{Title: "Union Station farmers market", URL: "https://www.denverunionstation.com/", Snippet: "Local produce, bread and coffee.", Date: day(now, 4), Label: "Union Station"},
					}
				},
			},
			{
				email:     "sam@rivera.test",
				parent:    true,
				role:      "parent; default page, moved standup on the calendar",
				interests: "Colorado Rockies, trail running, home coffee roasting",
				location:  "Denver, CO",
				calendar:  "sam",
				articles: func(now time.Time) []llm.Item {
					return []llm.Item{
						{Title: "Rockies announce spring training schedule", URL: "https://www.mlb.com/rockies", Snippet: "Cactus League games start in late February.", Date: day(now, 0), Label: "MLB.com"},
						{Title: "Best fall trail runs near Denver", URL: "https://www.alltrails.com/", Snippet: "Aspen-lined loops from 5 to 12 miles.", Date: day(now, -1), Label: "AllTrails"},
						{Title: "Roasting coffee at home with a popcorn popper", URL: "https://www.sweetmarias.com/", Snippet: "A cheap way to start, and how to tell first crack from second.", Date: day(now, -3), Label: "Sweet Maria's"},
					}
				},
				events: func(now time.Time) []llm.Item {
					return []llm.Item{
						{Title: "Rockies fan fest at Coors Field", URL: "https://www.mlb.com/rockies", Snippet: "Autographs, batting cages and stadium tours.", Date: day(now, 5), Label: "Coors Field"},
						{Title: "Wash Park 5K", URL: "https://www.runningusa.org/", Snippet: "A flat loop around Washington Park.", Date: day(now, 3) + ", 8:00 AM", Label: "Washington Park"},
						{Title: "Coffee cupping at a local roaster", URL: "https://www.sweetmarias.com/", Snippet: "Taste three single-origin coffees side by side.", Date: day(now, 2), Label: "RiNo"},
					}
				},
			},
			{
				email: "jordan@rivera.test",
				role:  "kid (15); kid-safe widget, deleted band practice on the calendar, two sample chats",
				age:   15,
				chats: []demoChat{
					{
						mode: "homework", title: "Solving 2x + 7 = 19",
						turns: []string{
							"can you just tell me the answer to 2x + 7 = 19",
							"I'm here to help you work it out rather than hand it over, and you've got this. What do you think the first step is to get x by itself?",
							"subtract 7?",
							"Exactly. Subtract 7 from both sides. What does the equation look like now?",
							"2x = 12 so x = 6",
							"That's right: x = 6. You undid the + 7 first, then the × 2. Want to try one more on your own?",
						},
					},
					{
						mode: "explore", title: "What makes a game level fun",
						turns: []string{
							"how do game designers make levels fun?",
							"Good question. A lot of it is pacing: a level teaches one new idea safely, then tests it, then twists it. Designers also leave little rewards off the main path for curious players. What kind of game are you thinking about making?",
						},
					},
				},
				interests: "soccer, video game design, robotics club",
				location:  "Denver, CO",
				calendar:  "jordan",
				widgets: []demoWidget{
					{
						title: "Robotics Competitions", source: "news_search", query: "FIRST robotics competition news",
						prompt: "news about FIRST robotics competitions", archetype: llm.ArchetypeList,
						items: func(now time.Time) []llm.Item {
							return []llm.Item{
								{Title: "FIRST Tech Challenge season kicks off", URL: "https://www.firstinspires.org/", Snippet: "Teams get this year's game and start building.", Date: day(now, -2), Label: "FIRST"},
								{Title: "Colorado robotics qualifier dates announced", URL: "https://www.firstinspires.org/", Snippet: "Regional qualifiers run from December through February.", Date: day(now, -5), Label: "FIRST"},
							}
						},
					},
				},
				articles: func(now time.Time) []llm.Item {
					return []llm.Item{
						{Title: "Colorado Rapids academy holds open tryouts", URL: "https://www.coloradorapids.com/", Snippet: "Youth teams for ages 13 to 17.", Date: day(now, 0), Label: "Colorado Rapids"},
						{Title: "How indie game designers prototype in a weekend", URL: "https://www.gamedeveloper.com/", Snippet: "Paper prototypes, game jams and free engines.", Date: day(now, -2), Label: "Game Developer"},
						{Title: "Student robotics team builds a rover", URL: "https://www.firstinspires.org/", Snippet: "A high school team's rover climbs stairs on its own.", Date: day(now, -4), Label: "FIRST"},
					}
				},
				events: func(now time.Time) []llm.Item {
					return []llm.Item{
						{Title: "Youth soccer skills clinic", URL: "https://www.coloradorapids.com/", Snippet: "For ages 12 to 16.", Date: day(now, 3) + ", 10:00 AM", Label: "Dick's Sporting Goods Park"},
						{Title: "Teen game design workshop", URL: "https://www.denverlibrary.org/", Snippet: "Make a simple game in an afternoon. Free, registration required.", Date: day(now, 4), Label: "Denver Public Library"},
					}
				},
			},
			{
				email: "lily@rivera.test",
				role:  "kid (9); no calendar, one sample chat with a flag to review",
				age:   9,
				chats: []demoChat{
					{
						mode: "explore", title: "Drawing a horse",
						turns: []string{
							"can you help me draw a horse?",
							"Yes! Start with two circles for the body, one big and one smaller, then a smaller circle for the head. Want to try that first?",
							"a girl in my drawing game asked where i live so she can mail me a horse sticker",
							"That's kind of her, but where you live is private, so it's best not to share it online, even with someone friendly. Could you tell a parent about it? They can help you decide what's safe. Want to keep working on your horse?",
						},
						flag: &demoFlag{turn: 2, category: "strangers", severity: "medium",
							reason: "Someone in an online drawing game asked Lily where she lives."},
					},
				},
				interests: "horses, drawing, dinosaurs",
				location:  "Denver, CO",
				articles: func(now time.Time) []llm.Item {
					return []llm.Item{
						{Title: "Scientists name a new plant-eating dinosaur", URL: "https://www.dmns.org/", Snippet: "Its fossils were found in Colorado rock.", Date: day(now, -1), Label: "Denver Museum of Nature & Science"},
						{Title: "How to draw a horse in five steps", URL: "https://www.artforkidshub.com/", Snippet: "Start with circles and connect them.", Date: day(now, -3), Label: "Art for Kids Hub"},
						{Title: "Meet the wild horses of the Colorado plateau", URL: "https://www.nationalgeographic.com/animals", Snippet: "How wild horse herds live together.", Date: day(now, -6), Label: "National Geographic"},
					}
				},
				events: func(now time.Time) []llm.Item {
					return []llm.Item{
						{Title: "Dinosaur discovery day", URL: "https://www.dmns.org/", Snippet: "Touch real fossils and meet a paleontologist.", Date: day(now, 3), Label: "Denver Museum of Nature & Science"},
						{Title: "Kids' drawing class", URL: "https://www.denverlibrary.org/", Snippet: "For ages 7 to 10. Supplies provided.", Date: day(now, 2) + ", 4:00 PM", Label: "Denver Public Library"},
					}
				},
			},
		},
		invites: []demoInvite{
			{email: "grandma@rivera.test", parent: true, role: "invited as a parent, hasn't joined yet"},
		},
	},
	{
		name: "Chen Family",
		members: []demoMember{
			{
				email:     "alex@chen.test",
				parent:    true,
				role:      "parent in a second family, for isolation checks",
				interests: "jazz, woodworking",
				location:  "Portland, OR",
				calendar:  "alex",
				widgets: []demoWidget{
					{
						title: "Portland Jazz Shows", source: "events_search", query: "jazz concerts Portland",
						prompt: "jazz shows in Portland", archetype: llm.ArchetypeList,
						items: func(now time.Time) []llm.Item {
							return []llm.Item{
								{Title: "Jazz night at the Old Church", URL: "https://www.theoldchurch.org/", Snippet: "A quartet plays standards.", Date: day(now, 2) + ", 7:30 PM", Label: "The Old Church"},
							}
						},
					},
				},
				articles: func(now time.Time) []llm.Item {
					return []llm.Item{
						{Title: "A beginner's guide to hand-cut dovetails", URL: "https://www.finewoodworking.com/", Snippet: "Saw, chop and pare.", Date: day(now, -1), Label: "Fine Woodworking"},
					}
				},
				events: func(now time.Time) []llm.Item {
					return []llm.Item{
						{Title: "Woodworking open shop", URL: "https://www.multnomahlibrary.org/", Snippet: "Bring a project.", Date: day(now, 4), Label: "Portland"},
					}
				},
			},
			{
				email:     "riley@chen.test",
				role:      "kid in the second family; no calendar",
				interests: "swimming, comic books",
				location:  "Portland, OR",
				articles: func(now time.Time) []llm.Item {
					return []llm.Item{
						{Title: "Free Comic Book Day picks", URL: "https://www.freecomicbookday.com/", Snippet: "All-ages comics to look for.", Date: day(now, -2), Label: "Free Comic Book Day"},
					}
				},
				events: func(now time.Time) []llm.Item {
					return []llm.Item{
						{Title: "Family swim night", URL: "https://www.portland.gov/parks", Snippet: "Open swim for all ages.", Date: day(now, 1) + ", 6:00 PM", Label: "Portland Parks"},
					}
				},
			},
		},
	},
}

// denverWeek is a plausible week of Denver weather in the shape the weather
// widget shows: one row per day.
func denverWeek(now time.Time) []llm.Item {
	skies := []string{"Sunny", "Partly cloudy", "Mostly sunny", "Chance of showers", "Sunny", "Windy", "Partly cloudy"}
	highs := []int{68, 64, 71, 58, 62, 66, 69}
	rain := []int{0, 10, 5, 40, 10, 0, 5}
	items := make([]llm.Item, len(skies))
	for i := range skies {
		name := now.AddDate(0, 0, i).Format("Monday")
		switch i {
		case 0:
			name = "Today"
		case 1:
			name = "Tomorrow"
		}
		items[i] = llm.Item{
			Title:   name,
			Date:    day(now, i),
			Label:   skies[i],
			Value:   fmt.Sprintf("%d° / %d°F", highs[i], highs[i]-26),
			Snippet: fmt.Sprintf("%d%% chance of rain", rain[i]),
		}
	}
	return items
}

// mayaLayout is Maya's own arrangement: the weather beside Today, the
// launches beside Articles, and the dinner ideas across the bottom.
// widgetIDs are Maya's weather, launches and dinner widgets, in that order.
func mayaLayout(widgetIDs []int64) layout.Data {
	weather, launches, dinners := widgetIDs[0], widgetIDs[1], widgetIDs[2]
	custom := func(id int64, span int) layout.Block {
		return layout.Block{Type: layout.TypeCustom, Props: layout.BlockProps{ID: fmt.Sprintf("custom-%d", id), Span: span, WidgetID: id}}
	}
	blocks := []layout.Block{
		{Type: layout.TypeBriefing, Props: layout.BlockProps{ID: "briefing", Span: 4}},
		{Type: layout.TypeToday, Props: layout.BlockProps{ID: "today", Span: 2}},
		custom(weather, 2),
		{Type: layout.TypeArticles, Props: layout.BlockProps{ID: "articles", Span: 2}},
		custom(launches, 2),
		{Type: layout.TypeLocalEvents, Props: layout.BlockProps{ID: "events", Span: 2}},
		custom(dinners, 2),
	}
	return layout.Data{
		Root:    layout.Root{Props: layout.RootProps{Blocks: blocks}},
		Content: []layout.Block{},
		Zones:   map[string][]layout.Block{},
	}
}
