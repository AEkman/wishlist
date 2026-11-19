package web

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/cucumber/godog"
	"golang.org/x/net/html"
)

func (a *acceptance) registerNavigationSteps(sc *godog.ScenarioContext) {
	sc.Given(`^the app has a child named "([^"]+)"$`, func(child string) error {
		_, err := a.db.CreateChild(a.ctx, child)
		return err
	})
	sc.When(`^I open the app home page$`, func() error { return a.get("/") })
	sc.Then(`^I can select (.+)'s child card immediately$`, func(child string) error {
		if err := a.clickLink(child); err != nil {
			return err
		}
		return a.expectScreen("lists")
	})
	sc.When(`^I create child cards for "([^"]+)" and "([^"]+)"$`, func(first, second string) error {
		for _, child := range []string{first, second} {
			if err := a.postForm("/children", url.Values{"name": {child}}); err != nil {
				return err
			}
			if err := a.follow(); err != nil {
				return err
			}
		}
		return nil
	})
	sc.When(`^I create the wishlists "([^"]+)" and "([^"]+)" for "([^"]+)"$`, func(first, second, child string) error {
		return a.createWishlistsFromApp(child, first, second)
	})
	sc.When(`^I create the wishlist "([^"]+)" for "([^"]+)"$`, func(title, child string) error {
		return a.createWishlistsFromApp(child, title)
	})
	sc.Then(`^(.+)'s card opens a screen with (?:only )?"([^"]+)"(?: and "([^"]+)")?$`, a.expectWishlistCards)
	sc.Given(`^(.+) has a wishlist with at least two wishes$`, func(child string) error {
		if err := a.seedWishlist(child, "Jul"); err != nil {
			return err
		}
		return a.seedWishes("Bike", "LEGO")
	})
	sc.When(`^I select (.+)'s child card$`, a.openChildCard)
	sc.Then(`^I see large cards for (.+)'s wishlists$`, func(child string) error {
		if err := a.expectScreen("lists"); err != nil {
			return err
		}
		headings := htmlNodes(a.doc, isElement("h1"))
		if len(headings) != 1 || htmlText(headings[0]) != child || len(a.choiceCards()) == 0 {
			return fmt.Errorf("expected %q's wishlist cards", child)
		}
		return nil
	})
	sc.When(`^I select one of those cards$`, func() error {
		cards := a.choiceCards()
		if len(cards) == 0 {
			return fmt.Errorf("no wishlist card to select")
		}
		return a.get(htmlAttr(cards[0], "href"))
	})
	sc.Then(`^I see the heading "([^"]+)" and two wishes from that wishlist$`, func(heading string) error {
		headings := htmlNodes(a.doc, isElement("h1"))
		if len(headings) != 1 || htmlText(headings[0]) != heading {
			return fmt.Errorf("expected heading %q", heading)
		}
		return a.expectPair("Bike", "LEGO")
	})
	sc.Then(`^each of the two screen transitions uses a 300 ms fade$`, a.expectFade)
	sc.Given(`^the wishlist contains fewer than two wishes$`, func() error { return a.seedWishlist("Alex", "Jul") })
	sc.Then(`^I can add a wish$`, func() error {
		inspection := *a
		if err := inspection.clickLink("Lägg till en önskan"); err != nil {
			return err
		}
		return inspection.expectScreen("add")
	})
	sc.Then(`^no comparison is shown$`, func() error {
		if len(a.compareForms()) != 0 {
			return fmt.Errorf("comparison forms are visible")
		}
		return nil
	})
}

func (a *acceptance) expectScreen(want string) error {
	screens := htmlNodes(a.doc, func(node *html.Node) bool { return htmlAttr(node, "id") == "screen" })
	if len(screens) != 1 || htmlAttr(screens[0], "data-screen") != want {
		return fmt.Errorf("expected screen %q", want)
	}
	return nil
}

func (a *acceptance) openChildCard(child string) error {
	if err := a.get("/"); err != nil {
		return err
	}
	return a.clickLink(child)
}

func (a *acceptance) createWishlistsFromApp(child string, titles ...string) error {
	for _, title := range titles {
		if err := a.openChildCard(child); err != nil {
			return err
		}
		forms := htmlNodes(a.doc, func(node *html.Node) bool {
			return isElement("form")(node) && formValues(node).Has("title")
		})
		if len(forms) != 1 {
			return fmt.Errorf("expected one wishlist creation form, found %d", len(forms))
		}
		if err := a.postForm(htmlAttr(forms[0], "action"), url.Values{"title": {title}}); err != nil {
			return err
		}
		if err := a.follow(); err != nil {
			return err
		}
	}
	return nil
}

func (a *acceptance) choiceCards() []*html.Node {
	return htmlNodes(a.doc, func(node *html.Node) bool {
		return isElement("a")(node) && hasClass(node, "choice-card")
	})
}

func (a *acceptance) expectWishlistCards(child, first, second string) error {
	if err := a.openChildCard(child); err != nil {
		return err
	}
	want := []string{first}
	if second != "" {
		want = append(want, second)
	}
	var got []string
	for _, card := range a.choiceCards() {
		got = append(got, htmlText(card))
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		return fmt.Errorf("%q's wishlist cards are %v, want %v", child, got, want)
	}
	return nil
}

func (a *acceptance) expectFade() error {
	if len(htmlNodes(a.doc, func(node *html.Node) bool { return hasClass(node, "screen-motion") })) != 1 {
		return fmt.Errorf("screen has no fade class")
	}
	if err := a.get("/static/app.css"); err != nil {
		return err
	}
	if !strings.Contains(a.body, "--fade-duration: 300ms") {
		return fmt.Errorf("stylesheet has no 300 ms fade duration")
	}
	return nil
}

func hasClass(node *html.Node, class string) bool {
	return slices.Contains(strings.Fields(htmlAttr(node, "class")), class)
}
