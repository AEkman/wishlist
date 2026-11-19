Feature: Share wishlists and record purchases

  Scenario: Find the share link from the ranked view
    Given I have opened the ranked view for a wishlist
    When I ask to share the wishlist
    Then I can copy its UUID link
    And the link opens the same wishlist without going through the child and wishlist screens

  Scenario: Open the full shared wishlist directly
    Given a wishlist with twelve wishes has a share link at its UUID path
    When I open the share link
    Then all twelve wishes appear in rank order
    And the first five wishes have larger cards than the rest
    And I can mark a wish as bought
    And the shared page has no controls to add, edit, or compare wishes

  Scenario: Show changes to the ranking on the shared wishlist
    Given I have opened the shared wishlist
    When a pick changes its ranking in the app
    And I reload the shared page
    Then the wishes appear in the new rank order

  Scenario: Mark a wish as bought with a name
    Given I have opened the shared wishlist
    When I select the purchase checkbox for "LEGO"
    Then the page asks for my name
    And I cannot save the purchase mark with a blank name
    When I save the purchase mark with the name "Anna"
    Then the card shows an overlay with "Köpt" on the first line
    And the overlay shows "av Anna" on the line below
    And another visitor who opens the same share link sees the purchase mark

  Scenario: Keep purchase marks off the comparison and ranked screens
    Given "LEGO" is marked as bought by "Anna" on the shared list
    When I compare "LEGO" with another wish in the app
    Then the comparison shows no purchase status for "LEGO"
    When I select "Visa önskelistan"
    Then the ranked view shows no purchase status for "LEGO"

  Scenario: Clear a purchase mark from the shared wishlist
    Given I have opened a shared wishlist with "LEGO" marked as bought by "Anna"
    When I clear the purchase checkbox for "LEGO"
    Then the page asks "Avmarkera som köpt?"
    When I confirm
    Then the overlay disappears
    And another visitor who opens the same share link sees "LEGO" as available
