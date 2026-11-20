package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
	"golang.org/x/net/html"
)

func (a *acceptance) registerSharingSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I have opened the ranked view for a wishlist$`, func() error {
		if err := a.seedSharingWishlist(2); err != nil {
			return err
		}
		return a.get(a.listPath() + "/ranked")
	})
	sc.Step(`^I ask to share the wishlist$`, func() error {
		return a.expectSharingText("summary", "Dela önskelistan")
	})
	sc.Step(`^I can copy its UUID link$`, func() error {
		_, err := a.visibleShareLink()
		return err
	})
	sc.Step(`^the link opens the same wishlist without going through the child and wishlist screens$`, func() error {
		link, err := a.visibleShareLink()
		if err != nil {
			return err
		}
		if err := a.get(link.Path); err != nil {
			return err
		}
		return a.expectSharingText("h1", a.wishlist.Title)
	})
	sc.Step(`^a wishlist with twelve wishes has a share link at its UUID path$`, func() error {
		if err := a.seedSharingWishlist(12); err != nil {
			return err
		}
		return uuid.Validate(a.wishlist.ShareID)
	})
	sc.Step(`^I open the share link$`, a.openSharedWishlist)
	sc.Step(`^all twelve wishes appear in rank order$`, func() error {
		return a.expectSharedRanks(sharingWishTitles(12)...)
	})
	sc.Step(`^I can mark a wish as bought$`, func() error {
		_, err := a.purchaseCheckboxAction("LEGO", "buy")
		return err
	})
	sc.Step(`^the shared page has no controls to add, edit, or compare wishes$`, func() error {
		controls := htmlNodes(a.doc, func(node *html.Node) bool {
			return strings.HasPrefix(htmlAttr(node, "href"), "/lists/") ||
				strings.HasPrefix(htmlAttr(node, "action"), "/lists/") ||
				strings.HasPrefix(htmlAttr(node, "hx-post"), "/lists/")
		})
		if len(controls) != 0 {
			return fmt.Errorf("found %d controls for editing or comparing wishes on the shared list", len(controls))
		}
		return nil
	})
	sc.Step(`^I have opened the shared wishlist$`, func() error {
		if err := a.seedSharingWishlist(2); err != nil {
			return err
		}
		return a.openSharedWishlist()
	})
	sc.Step(`^a pick changes its ranking in the app$`, func() error {
		winner, err := a.findWish("LEGO")
		if err != nil {
			return err
		}
		loser, err := a.findWish("Bike")
		if err != nil {
			return err
		}
		if err := a.postForm(a.listPath()+"/pick", url.Values{"winner": {winner.ID}, "loser": {loser.ID}}); err != nil {
			return err
		}
		return a.follow()
	})
	sc.Step(`^I reload the shared page$`, a.openSharedWishlist)
	sc.Step(`^the wishes appear in the new rank order$`, func() error {
		return a.expectSharedRanks("LEGO", "Bike")
	})
	sc.Step(`^I select the purchase checkbox for "([^"]+)"$`, func(title string) error {
		return a.openPurchaseCheckbox(title, "buy")
	})
	sc.Step(`^the page asks for my name$`, func() error {
		names := htmlNodes(a.doc, func(node *html.Node) bool {
			return node.Data == "input" && htmlAttr(node, "name") == "name"
		})
		if len(names) != 1 {
			return fmt.Errorf("expected one buyer name input, found %d", len(names))
		}
		return a.expectSharingText("label", "Ditt namn")
	})
	sc.Step(`^I cannot save the purchase mark with a blank name$`, a.rejectBlankBuyer)
	sc.Step(`^I save the purchase mark with the name "([^"]+)"$`, func(buyer string) error {
		return a.submitPurchaseForm(url.Values{"name": {buyer}})
	})
	sc.Step(`^the card shows an overlay with "([^"]+)" on the first line$`, func(headline string) error {
		return a.expectPurchaseOverlay("strong", headline)
	})
	sc.Step(`^the overlay shows "([^"]+)" on the line below$`, func(byline string) error {
		return a.expectPurchaseOverlay("span", byline)
	})
	sc.Step(`^another visitor who opens the same share link sees the purchase mark$`, func() error {
		if err := a.openSharedWishlist(); err != nil {
			return err
		}
		return a.expectPurchaseOverlay("span", "av Anna")
	})
	sc.Step(`^"([^"]+)" is marked as bought by "([^"]+)" on the shared list$`, a.seedPurchase)
	sc.Step(`^I compare "([^"]+)" with another wish in the app$`, func(title string) error {
		if err := a.get(a.listPath()); err != nil {
			return err
		}
		return a.expectSharingText("span", title)
	})
	sc.Step(`^the (?:comparison|ranked view) shows no purchase status for "([^"]+)"$`, func(title string) error {
		if !strings.Contains(a.body, title) {
			return fmt.Errorf("screen does not show wish %q", title)
		}
		return a.expectNoPurchaseOverlay()
	})
	sc.Step(`^I have opened a shared wishlist with "([^"]+)" marked as bought by "([^"]+)"$`, func(title, buyer string) error {
		if err := a.seedPurchase(title, buyer); err != nil {
			return err
		}
		return a.openSharedWishlist()
	})
	sc.Step(`^I clear the purchase checkbox for "([^"]+)"$`, func(title string) error {
		return a.openPurchaseCheckbox(title, "clear")
	})
	sc.Step(`^the page asks "Avmarkera som köpt\?"$`, func() error {
		return a.expectSharingText("h1", "Avmarkera som köpt?")
	})
	sc.Step(`^I confirm$`, func() error { return a.submitPurchaseForm(nil) })
	sc.Step(`^the overlay disappears$`, a.expectNoPurchaseOverlay)
	sc.Step(`^another visitor who opens the same share link sees "([^"]+)" as available$`, func(title string) error {
		if err := a.openSharedWishlist(); err != nil {
			return err
		}
		_, err := a.purchaseCheckboxAction(title, "buy")
		return err
	})
}

func (a *acceptance) seedSharingWishlist(count int) error {
	if err := a.seedWishlist("Alex", "Jul"); err != nil {
		return err
	}
	for _, title := range sharingWishTitles(count) {
		if _, err := a.seedWish(title); err != nil {
			return err
		}
	}
	return nil
}

func sharingWishTitles(count int) []string {
	titles := []string{"Bike", "LEGO"}
	for index := 2; index < count; index++ {
		titles = append(titles, fmt.Sprintf("Wish %d", index+1))
	}
	return titles
}

func (a *acceptance) seedPurchase(title, buyer string) error {
	if err := a.seedSharingWishlist(2); err != nil {
		return err
	}
	wish, err := a.findWish(title)
	if err != nil {
		return err
	}
	return a.db.MarkBought(a.ctx, a.wishlist.ID, wish.ID, buyer)
}

func (a *acceptance) openSharedWishlist() error {
	return a.get("/" + a.wishlist.ShareID)
}

func (a *acceptance) visibleShareLink() (*url.URL, error) {
	inputs := htmlNodes(a.doc, func(node *html.Node) bool {
		return node.Data == "input" && htmlAttr(node, "id") == "share-url"
	})
	if len(inputs) != 1 {
		return nil, fmt.Errorf("expected a share link field, found %d", len(inputs))
	}
	link, err := url.Parse(htmlAttr(inputs[0], "value"))
	if err != nil {
		return nil, err
	}
	if link.String() != acceptanceBaseURL+"/"+a.wishlist.ShareID || uuid.Validate(a.wishlist.ShareID) != nil {
		return nil, fmt.Errorf("expected a share link with UUID path, got %q", link)
	}
	return link, nil
}

func (a *acceptance) sharedWishCards() []*html.Node {
	return htmlNodes(a.doc, func(node *html.Node) bool {
		return node.Data == "li" && htmlAttr(node, "data-rank") != ""
	})
}

func (a *acceptance) expectSharedRanks(wants ...string) error {
	cards := a.sharedWishCards()
	if len(cards) != len(wants) {
		return fmt.Errorf("expected %d ranked cards, found %d", len(wants), len(cards))
	}
	for index, card := range cards {
		rank := strconv.Itoa(index + 1)
		if htmlAttr(card, "data-rank") != rank {
			return fmt.Errorf("expected rank %s, got %q", rank, htmlAttr(card, "data-rank"))
		}
		if err := expectSharingElementText(card, "h2", wants[index]); err != nil {
			return fmt.Errorf("rank %s: %w", rank, err)
		}
	}
	return nil
}

func (a *acceptance) purchaseCheckboxAction(title, action string) (string, error) {
	wish, err := a.findWish(title)
	if err != nil {
		return "", err
	}
	path := "/" + a.wishlist.ShareID + "/wishes/" + wish.ID + "/" + action
	controls := htmlNodes(a.doc, func(node *html.Node) bool {
		return node.Data == "input" && htmlAttr(node, "type") == "checkbox" && htmlAttr(node, "hx-get") == path
	})
	if len(controls) != 1 {
		return "", fmt.Errorf("expected purchase checkbox for %q, found %d", title, len(controls))
	}
	checked := slices.ContainsFunc(controls[0].Attr, func(attr html.Attribute) bool {
		return attr.Key == "checked"
	})
	if checked != (action == "clear") {
		return "", fmt.Errorf("purchase checkbox for %q has incorrect checked state", title)
	}
	return path, nil
}

func (a *acceptance) openPurchaseCheckbox(title, action string) error {
	path, err := a.purchaseCheckboxAction(title, action)
	if err != nil {
		return err
	}
	return a.get(path)
}

func (a *acceptance) currentPurchaseForm() (string, error) {
	forms := htmlNodes(a.doc, func(node *html.Node) bool {
		return node.Data == "form" && htmlAttr(node, "method") == "post"
	})
	if len(forms) != 1 {
		return "", fmt.Errorf("expected one purchase form, found %d", len(forms))
	}
	return htmlAttr(forms[0], "action"), nil
}

func (a *acceptance) rejectBlankBuyer() error {
	path, err := a.currentPurchaseForm()
	if err != nil {
		return err
	}
	if err := a.postForm(path, url.Values{"name": {""}}); err != nil {
		return err
	}
	switch a.response.StatusCode {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return a.get(path)
	default:
		return fmt.Errorf("blank buyer returned HTTP %d, expected an input error", a.response.StatusCode)
	}
}

func (a *acceptance) submitPurchaseForm(values url.Values) error {
	path, err := a.currentPurchaseForm()
	if err != nil {
		return err
	}
	if err := a.postForm(path, values); err != nil {
		return err
	}
	return a.follow()
}

func (a *acceptance) purchaseOverlays() []*html.Node {
	return htmlNodes(a.doc, func(node *html.Node) bool {
		return slices.Contains(strings.Fields(htmlAttr(node, "class")), "purchase-overlay")
	})
}

func (a *acceptance) expectPurchaseOverlay(tag, text string) error {
	overlays := a.purchaseOverlays()
	if len(overlays) != 1 {
		return fmt.Errorf("expected one purchase overlay, found %d", len(overlays))
	}
	return expectSharingElementText(overlays[0], tag, text)
}

func (a *acceptance) expectNoPurchaseOverlay() error {
	if len(a.purchaseOverlays()) != 0 || strings.Contains(a.body, "Anna") {
		return errors.New("screen still shows a purchase mark")
	}
	return nil
}

func (a *acceptance) expectSharingText(tag, text string) error {
	return expectSharingElementText(a.doc, tag, text)
}

func expectSharingElementText(root *html.Node, tag, text string) error {
	nodes := htmlNodes(root, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == tag && htmlText(node) == text
	})
	if len(nodes) == 0 {
		return fmt.Errorf("expected %q in a %s element", text, tag)
	}
	return nil
}
