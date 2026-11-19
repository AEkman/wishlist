package web

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"golang.org/x/net/html"
)

func (a *acceptance) registerRankingSteps(sc *godog.ScenarioContext) {
	sc.Given(`^the wishlist has more than ten wishes$`, func() error {
		a.draws = []int{11, 10}
		return a.seedNumberedWishes(12)
	})
	sc.When(`^I open the wishlist$`, func() error { return a.get(a.listPath()) })
	sc.Then(`^I see two different wishes drawn at random from the entire wishlist$`, func() error {
		return a.expectPair("Wish 12", "Wish 11")
	})
	sc.Then(`^the ranked list is hidden$`, func() error {
		if len(a.rankedCards()) != 0 {
			return fmt.Errorf("ranked wish cards are visible")
		}
		return nil
	})
	sc.Given(`^the wishlist contains only "([^"]+)" and "([^"]+)"$`, func(first, second string) error {
		if err := a.seedWishlist("Alex", "Jul"); err != nil {
			return err
		}
		a.draws = []int{1, 0}
		return a.seedWishes(first, second)
	})
	sc.Then(`^I see both wishes in random left-to-right order$`, func() error {
		return a.expectPair("LEGO", "Bike")
	})
	sc.Then(`^I can pick either wish$`, func() error {
		for _, form := range a.compareForms() {
			buttons := htmlNodes(form, isElement("button"))
			if len(buttons) != 1 || htmlAttr(buttons[0], "type") != "submit" {
				return fmt.Errorf("wish has no comparison button")
			}
		}
		return nil
	})
	sc.Given(`^the wishlist order is "([^"]+)"$`, func(order string) error {
		if err := a.seedWishlist("Alex", "Jul"); err != nil {
			return err
		}
		return a.seedWishes(strings.Split(order, ", ")...)
	})
	sc.Given(`^I am comparing "([^"]+)" and "([^"]+)"$`, a.openPair)
	sc.When(`^I pick "([^"]+)"$`, a.pick)
	sc.Then(`^the wishlist order is (?:immediately )?"([^"]+)"$`, a.expectWishlistOrder)
	sc.Given(`^I am comparing wishes in a wishlist with twelve wishes$`, func() error {
		if err := a.seedNumberedWishes(12); err != nil {
			return err
		}
		return a.get(a.listPath())
	})
	sc.When(`^I select "Visa önskelistan"$`, func() error { return a.clickLink("Visa önskelistan") })
	sc.Then(`^I see ranks 1 through 10 in order$`, func() error {
		if err := a.expectScreen("ranked"); err != nil {
			return err
		}
		cards := a.rankedCards()
		if len(cards) != 10 {
			return fmt.Errorf("found %d ranked cards, want 10", len(cards))
		}
		for i, card := range cards {
			if got := htmlAttr(card, "data-rank"); got != strconv.Itoa(i+1) {
				return fmt.Errorf("card %d has rank %q", i+1, got)
			}
		}
		return nil
	})
	sc.Then(`^the first five wishes have larger cards than the next five$`, func() error {
		for i, card := range a.rankedCards() {
			if hasClass(card, "ranked-card--large") != (i < 5) {
				return fmt.Errorf("wrong card size at rank %d", i+1)
			}
		}
		return nil
	})
	sc.Then(`^the comparison is hidden$`, func() error {
		if len(a.compareForms()) != 0 {
			return fmt.Errorf("comparison forms are visible")
		}
		return nil
	})
}

func (a *acceptance) seedWishes(titles ...string) error {
	for _, title := range titles {
		if _, err := a.seedWish(title); err != nil {
			return err
		}
	}
	return nil
}

func (a *acceptance) seedNumberedWishes(count int) error {
	if err := a.seedWishlist("Alex", "Jul"); err != nil {
		return err
	}
	for i := range count {
		if _, err := a.seedWish("Wish " + strconv.Itoa(i+1)); err != nil {
			return err
		}
	}
	return nil
}

func (a *acceptance) openPair(left, right string) error {
	wishes, err := a.db.Wishes(a.ctx, a.wishlist.ID)
	if err != nil {
		return err
	}
	leftIndex, rightIndex := -1, -1
	for i, wish := range wishes {
		switch wish.Title {
		case left:
			leftIndex = i
		case right:
			rightIndex = i
		}
	}
	if leftIndex < 0 || rightIndex < 0 {
		return fmt.Errorf("cannot compare %q and %q", left, right)
	}
	if rightIndex > leftIndex {
		rightIndex--
	}
	a.draws = []int{leftIndex, rightIndex}
	if err := a.get(a.listPath()); err != nil {
		return err
	}
	return a.expectPair(left, right)
}

func (a *acceptance) expectPair(first, second string) error {
	forms := a.compareForms()
	if len(forms) != 2 {
		return fmt.Errorf("found %d comparison choices, want 2", len(forms))
	}
	for i, title := range []string{first, second} {
		wish, err := a.findWish(title)
		if err != nil {
			return err
		}
		if formValues(forms[i]).Get("winner") != wish.ID {
			return fmt.Errorf("choice %d does not show %q", i+1, title)
		}
	}
	return nil
}

func (a *acceptance) compareForms() []*html.Node {
	return htmlNodes(a.doc, func(node *html.Node) bool {
		return isElement("form")(node) && htmlAttr(node, "action") == a.listPath()+"/pick"
	})
}

func (a *acceptance) pick(title string) error {
	wish, err := a.findWish(title)
	if err != nil {
		return err
	}
	for _, form := range a.compareForms() {
		values := formValues(form)
		if values.Get("winner") != wish.ID {
			continue
		}
		if err := a.postForm(htmlAttr(form, "action"), values); err != nil {
			return err
		}
		if a.response.StatusCode == http.StatusSeeOther {
			return a.follow()
		}
		return a.expectStatus(http.StatusOK)
	}
	return fmt.Errorf("no comparison choice for %q", title)
}

func (a *acceptance) expectWishlistOrder(order string) error {
	inspection := *a
	if err := inspection.get(a.listPath() + "/ranked"); err != nil {
		return err
	}
	var got []string
	for _, card := range inspection.rankedCards() {
		for _, node := range htmlNodes(card, isElement("h2")) {
			got = append(got, htmlText(node))
		}
	}
	want := strings.Split(order, ", ")
	if !slices.Equal(got, want) {
		return fmt.Errorf("wishlist order is %v, want %v", got, want)
	}
	return nil
}

func (a *acceptance) rankedCards() []*html.Node {
	return htmlNodes(a.doc, func(node *html.Node) bool {
		return isElement("li")(node) && hasClass(node, "ranked-card")
	})
}

func formValues(form *html.Node) url.Values {
	values := make(url.Values)
	for _, input := range htmlNodes(form, isElement("input")) {
		if name := htmlAttr(input, "name"); name != "" {
			values.Add(name, htmlAttr(input, "value"))
		}
	}
	return values
}
