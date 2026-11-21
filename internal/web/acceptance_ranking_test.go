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
		return a.seedNumberedWishes(12)
	})
	sc.When(`^I open the wishlist$`, func() error { return a.get(a.listPath()) })
	sc.Then(`^I see a comparison between "([^"]+)" and "([^"]+)"$`, a.expectUnorderedPair)
	sc.Then(`^(\d+) of (\d+) wishes are placed$`, func(placed, total int) error {
		want := fmt.Sprintf("%d av %d önskningar placerade", placed, total)
		if !strings.Contains(htmlText(a.doc), want) {
			return fmt.Errorf("missing ranking progress %q", want)
		}
		return nil
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
		return a.seedWishes(first, second)
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
	sc.Then(`^ranking is complete$`, func() error {
		if err := a.expectScreen("ranked"); err != nil {
			return err
		}
		if !strings.Contains(htmlText(a.doc), "Rangordningen är klar") || len(a.compareForms()) != 0 {
			return fmt.Errorf("expected the completed ranked list")
		}
		return nil
	})
	sc.When(`^I undo the latest choice$`, func() error { return a.submitRankingControl("undo") })
	sc.When(`^I restart ranking$`, func() error { return a.submitRankingControl("restart") })
	sc.When(`^I continue ranking$`, func() error { return a.clickLink("Fortsätt rangordna") })
	sc.Given(`^the wishlist order is "([^"]+)"$`, func(order string) error {
		if err := a.seedWishlist("Alex", "Jul"); err != nil {
			return err
		}
		return a.seedWishes(strings.Split(order, ", ")...)
	})
	sc.When(`^I pick "([^"]+)"$`, a.pick)
	sc.Then(`^the wishlist order is (?:immediately )?"([^"]+)"$`, a.expectWishlistOrder)
	sc.Given(`^I am comparing wishes in a wishlist with twelve wishes$`, func() error {
		if err := a.seedNumberedWishes(12); err != nil {
			return err
		}
		return a.get(a.listPath())
	})
	sc.When(`^I select "Visa önskelistan"$`, func() error { return a.clickLink("Visa önskelistan") })
	sc.Then(`^I see ranks 1 through 12 in order$`, func() error {
		if err := a.expectScreen("ranked"); err != nil {
			return err
		}
		cards := a.rankedCards()
		if len(cards) != 12 {
			return fmt.Errorf("found %d ranked cards, want 12", len(cards))
		}
		for i, card := range cards {
			if got := htmlAttr(card, "data-rank"); got != strconv.Itoa(i+1) {
				return fmt.Errorf("card %d has rank %q", i+1, got)
			}
		}
		return nil
	})
	sc.Then(`^all wishes use the same card style$`, func() error {
		for i, card := range a.rankedCards() {
			if htmlAttr(card, "class") != "ranked-card" {
				return fmt.Errorf("rank %d has a different card style", i+1)
			}
		}
		return nil
	})
	sc.Then(`^the wishes appear in a single column$`, func() error {
		lists := htmlNodes(a.doc, func(node *html.Node) bool {
			return isElement("ol")(node) && hasClass(node, "ranked-list")
		})
		if len(lists) != 1 {
			return fmt.Errorf("want one ordered list for the vertical layout, got %d", len(lists))
		}
		for _, card := range a.rankedCards() {
			if card.Parent != lists[0] {
				return fmt.Errorf("a ranked wish is outside the ordered list")
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

func (a *acceptance) expectUnorderedPair(first, second string) error {
	forms := a.compareForms()
	if len(forms) != 2 {
		return fmt.Errorf("found %d comparison choices, want 2", len(forms))
	}
	for _, title := range []string{first, second} {
		wish, err := a.findWish(title)
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(forms, func(form *html.Node) bool {
			return formValues(form).Get("winner") == wish.ID
		}) {
			return fmt.Errorf("cannot choose %q", title)
		}
	}
	return nil
}

func (a *acceptance) submitRankingControl(action string) error {
	for _, form := range htmlNodes(a.doc, isElement("form")) {
		if htmlAttr(form, "action") != a.listPath()+"/ranking/"+action {
			continue
		}
		if err := a.postForm(htmlAttr(form, "action"), formValues(form)); err != nil {
			return err
		}
		return a.follow()
	}
	return fmt.Errorf("ranking control %q not found", action)
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
