package web

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/AEkman/wishlist/internal/store"
	"github.com/cucumber/godog"
)

const newWishTitle = "New wish"

type uploadAcceptance struct {
	*acceptance
	uploadedTitle         string
	uploadedDescription   string
	opponents             []string
	newWishAlwaysOnLeft   bool
	editStartedComparison bool
}

func (a *acceptance) registerUploadSteps(sc *godog.ScenarioContext) {
	u := &uploadAcceptance{acceptance: a}
	sc.Given(`^Alex's wishlist is empty$`, func() error { return u.openAlexWishlist(0) })
	sc.Given(`^I have opened Alex's wishlist$`, func() error { return u.openAlexWishlist(0) })
	sc.Given(`^Alex's wishlist has (\d+) wishes$`, u.openAlexWishlist)
	sc.When(`^I upload a picture with the title "([^"]+)" and leave the description blank$`, func(title string) error {
		return u.addPicture(title, "")
	})
	sc.When(`^I upload a picture with the title "([^"]+)" and the description "([^"]+)"$`, u.addPicture)
	sc.When(`^I open the add-wish form$`, func() error { return u.clickLink("Lägg till en önskan") })
	sc.Then(`^I can take a picture or choose an existing picture$`, u.checkPictureSources)
	sc.When(`^I submit a camera picture with the title "([^"]+)"$`, func(title string) error {
		u.uploadedTitle = title
		u.uploadedDescription = "From the camera"
		return u.submitPicture("camera", title, u.uploadedDescription, image.Rect(0, 0, 2, 1))
	})
	sc.When(`^I add a wish with a picture and title$`, func() error { return u.addPicture(newWishTitle, "") })
	sc.Then(`^"([^"]+)" is first$`, u.wishIsFirst)
	sc.Then(`^"([^"]+)" is the first wish on Alex's wishlist$`, u.wishIsFirst)
	sc.Then(`^Alex's wish includes the picture, title, and description$`, u.originalWishIsVisible)
	sc.Then(`^the upload requires (\d+) picks$`, u.finishUploadComparisons)
	sc.Then(`^the new wish stays on the left in each upload comparison$`, func() error {
		if !u.newWishAlwaysOnLeft {
			return fmt.Errorf("the new wish appeared on the right during upload")
		}
		return nil
	})
	sc.Then(`^each opponent appears at most once in the upload comparisons$`, u.opponentsAreUnique)
	sc.Given(`^the first upload comparison randomly selects "([^"]+)" as the opponent$`, u.drawUploadOpponent)
	sc.Then(`^the next opponent is one of "([^"]+)"$`, u.nextOpponentIsOneOf)
	sc.Given(`^"([^"]+)" is ranked third and marked as bought by "([^"]+)" on the shared list$`, u.seedBoughtThirdWish)
	sc.When(`^I change the wish's picture, title, and description$`, u.editBoughtWish)
	sc.Then(`^the updated wish is still ranked third$`, u.editedWishStillRanksThird)
	sc.Then(`^the shared list still shows it as bought by "([^"]+)"$`, u.editedWishStillShowsBuyer)
	sc.Then(`^the edit starts no upload comparisons$`, func() error {
		if u.editStartedComparison {
			return fmt.Errorf("editing a wish started upload comparisons")
		}
		return nil
	})
}

func (u *uploadAcceptance) openAlexWishlist(wishCount int) error {
	if err := u.seedWishlist("Alex", "Jul"); err != nil {
		return err
	}
	for i := range wishCount {
		if _, err := u.seedWish(fmt.Sprintf("Wish %d", i+1)); err != nil {
			return err
		}
	}
	return u.get(u.listPath())
}

func (u *uploadAcceptance) addPicture(title, description string) error {
	if err := u.clickLink("Lägg till en önskan"); err != nil {
		return err
	}
	u.uploadedTitle = title
	u.uploadedDescription = description
	return u.submitPicture("image", title, description, image.Rect(0, 0, 2, 1))
}

func (u *uploadAcceptance) checkPictureSources() error {
	sources := map[string]string{}
	for _, input := range htmlNodes(u.doc, isElement("input")) {
		if htmlAttr(input, "type") != "file" {
			continue
		}
		for _, attr := range input.Attr {
			if attr.Key == "required" {
				return fmt.Errorf("each picture source must work without the other source")
			}
		}
		sources[htmlAttr(input, "name")] = htmlAttr(input, "capture")
	}
	if len(sources) != 2 || sources["camera"] != "environment" || sources["image"] != "" {
		return fmt.Errorf("want a rear camera and a separate gallery input, got %v", sources)
	}
	return nil
}

func (u *uploadAcceptance) submitPicture(source, title, description string, bounds image.Rectangle) error {
	var action string
	for _, form := range htmlNodes(u.doc, isElement("form")) {
		if htmlAttr(form, "enctype") == "multipart/form-data" {
			action = htmlAttr(form, "action")
			break
		}
	}
	if action == "" {
		return fmt.Errorf("no picture upload form found")
	}

	var body bytes.Buffer
	upload := multipart.NewWriter(&body)
	if err := upload.WriteField("title", title); err != nil {
		return err
	}
	if err := upload.WriteField("description", description); err != nil {
		return err
	}
	part, err := upload.CreateFormFile(source, "wish.png")
	if err != nil {
		return err
	}
	if err := png.Encode(part, image.NewRGBA(bounds)); err != nil {
		return err
	}
	if err := upload.Close(); err != nil {
		return err
	}
	if err := u.request(http.MethodPost, action, &body, upload.FormDataContentType()); err != nil {
		return err
	}
	return u.follow()
}

func (u *uploadAcceptance) wishIsFirst(title string) error {
	wishes, err := u.db.Wishes(u.ctx, u.wishlist.ID)
	if err != nil {
		return err
	}
	if len(wishes) == 0 || wishes[0].Title != title {
		return fmt.Errorf("%q is not first on the wishlist", title)
	}
	return nil
}

func (u *uploadAcceptance) originalWishIsVisible() error {
	wish, err := u.findWish(u.uploadedTitle)
	if err != nil {
		return err
	}
	if !strings.Contains(u.body, u.uploadedTitle) || !strings.Contains(u.body, u.uploadedDescription) {
		return fmt.Errorf("the wish screen does not display the uploaded title and description")
	}
	return u.showsPicture(wish.ID, image.Rect(0, 0, 2, 1))
}

func (u *uploadAcceptance) showsPicture(wishID string, bounds image.Rectangle) error {
	path := "/images/" + wishID
	for _, img := range htmlNodes(u.doc, isElement("img")) {
		src, err := url.Parse(htmlAttr(img, "src"))
		if err != nil || src.IsAbs() || src.Host != "" || src.Path != path {
			continue
		}
		if err := u.get(src.RequestURI()); err != nil {
			return err
		}
		picture, err := png.Decode(strings.NewReader(u.body))
		if err != nil {
			return err
		}
		if picture.Bounds() != bounds {
			return fmt.Errorf("picture bounds %v, want %v", picture.Bounds(), bounds)
		}
		return nil
	}
	return fmt.Errorf("the wish screen does not display the uploaded picture")
}

func (u *uploadAcceptance) uploadPair() []string {
	var ids []string
	for _, form := range u.compareForms() {
		values := formValues(form)
		if values.Get("upload") != "" {
			ids = append(ids, values.Get("winner"))
		}
	}
	return ids
}

func (u *uploadAcceptance) finishUploadComparisons(picks int) error {
	wish, err := u.findWish(newWishTitle)
	if err != nil {
		return err
	}
	u.newWishAlwaysOnLeft = true
	u.opponents = nil
	for i := range picks {
		pair := u.uploadPair()
		if len(pair) != 2 || !slices.Contains(pair, wish.ID) {
			return fmt.Errorf("upload comparison %d does not offer two choices including the new wish", i+1)
		}
		u.newWishAlwaysOnLeft = u.newWishAlwaysOnLeft && pair[0] == wish.ID
		for _, id := range pair {
			if id != wish.ID {
				u.opponents = append(u.opponents, id)
			}
		}
		if err := u.pick(newWishTitle); err != nil {
			return err
		}
	}
	if len(u.uploadPair()) != 0 {
		return fmt.Errorf("upload still offers comparisons after %d picks", picks)
	}
	return nil
}

func (u *uploadAcceptance) opponentsAreUnique() error {
	for i, id := range u.opponents {
		if slices.Contains(u.opponents[:i], id) {
			return fmt.Errorf("opponent %q appeared more than once", id)
		}
	}
	return nil
}

func (u *uploadAcceptance) drawUploadOpponent(title string) error {
	newWish, err := u.findWish(newWishTitle)
	if err != nil {
		return err
	}
	wishes, err := u.db.Wishes(u.ctx, u.wishlist.ID)
	if err != nil {
		return err
	}
	wishes = slices.DeleteFunc(wishes, func(wish store.Wish) bool { return wish.ID == newWish.ID })
	draw := slices.IndexFunc(wishes, func(wish store.Wish) bool { return wish.Title == title })
	if draw < 0 {
		return fmt.Errorf("no opponent named %q", title)
	}
	u.draws = []int{draw}
	query := url.Values{"upload": {newWish.ID}}
	if err := u.get(u.listPath() + "?" + query.Encode()); err != nil {
		return err
	}
	return u.nextOpponentIsOneOf(title)
}

func (u *uploadAcceptance) nextOpponentIsOneOf(titles string) error {
	newWish, err := u.findWish(newWishTitle)
	if err != nil {
		return err
	}
	pair := u.uploadPair()
	if len(pair) != 2 || pair[0] != newWish.ID {
		return fmt.Errorf("expected two choices with the new wish on the left")
	}
	opponent, err := u.db.Wish(u.ctx, u.wishlist.ID, pair[1])
	if err != nil {
		return err
	}
	if !slices.Contains(strings.Split(titles, ", "), opponent.Title) {
		return fmt.Errorf("opponent is %q, want one of %q", opponent.Title, titles)
	}
	return nil
}

func (u *uploadAcceptance) seedBoughtThirdWish(title, buyer string) error {
	if err := u.openAlexWishlist(2); err != nil {
		return err
	}
	wish, err := u.seedWish(title)
	if err != nil {
		return err
	}
	return u.db.MarkBought(u.ctx, u.wishlist.ID, wish.ID, buyer)
}

const editedWishTitle = "Rymd-LEGO"
const editedWishDescription = "Det stora rymdsetet"

func (u *uploadAcceptance) editBoughtWish() error {
	wish, err := u.findWish("LEGO")
	if err != nil {
		return err
	}
	if err := u.get(u.listPath() + "/wishes/" + wish.ID + "/edit"); err != nil {
		return err
	}
	if err := u.submitPicture("image", editedWishTitle, editedWishDescription, image.Rect(0, 0, 3, 2)); err != nil {
		return err
	}
	u.editStartedComparison = len(u.uploadPair()) != 0
	return nil
}

func (u *uploadAcceptance) editedWishStillRanksThird() error {
	if err := u.get(u.listPath() + "/ranked"); err != nil {
		return err
	}
	wish, err := u.findWish(editedWishTitle)
	if err != nil {
		return err
	}
	for _, card := range htmlNodes(u.doc, isElement("li")) {
		if htmlAttr(card, "data-rank") == "3" &&
			strings.Contains(htmlText(card), editedWishTitle) &&
			strings.Contains(htmlText(card), editedWishDescription) {
			return u.showsPicture(wish.ID, image.Rect(0, 0, 3, 2))
		}
	}
	return fmt.Errorf("rank 3 does not show the updated title and description")
}

func (u *uploadAcceptance) editedWishStillShowsBuyer(buyer string) error {
	if err := u.get("/" + u.wishlist.ShareID); err != nil {
		return err
	}
	for _, card := range htmlNodes(u.doc, isElement("li")) {
		if !strings.Contains(htmlText(card), editedWishTitle) {
			continue
		}
		for _, line := range htmlNodes(card, isElement("span")) {
			if htmlText(line) == "av "+buyer {
				return nil
			}
		}
	}
	return fmt.Errorf("the shared list does not show the edited wish as bought by %q", buyer)
}
