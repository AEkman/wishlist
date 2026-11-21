Feature: Rank wishes by comparing two at a time

  Scenario: Start a guided ranking of the whole wishlist
    Given the wishlist has more than ten wishes
    When I open the wishlist
    Then I see a comparison between "Wish 1" and "Wish 2"
    And 1 of 12 wishes are placed
    And the ranked list is hidden

  Scenario: Undo the final choice and choose again
    Given the wishlist contains only "Bike" and "LEGO"
    When I open the wishlist
    Then I see a comparison between "Bike" and "LEGO"
    And I can pick either wish
    When I pick "LEGO"
    Then ranking is complete
    And the wishlist order is "LEGO, Bike"
    When I undo the latest choice
    Then I see a comparison between "Bike" and "LEGO"
    And 1 of 2 wishes are placed
    And the wishlist order is "Bike, LEGO"
    When I pick "Bike"
    Then ranking is complete
    And the wishlist order is "Bike, LEGO"
    When I restart ranking
    Then I see a comparison between "Bike" and "LEGO"
    And 1 of 2 wishes are placed

  Scenario: Narrow the insertion range until every wish has a place
    Given the wishlist order is "Bike, LEGO, Headphones"
    When I open the wishlist
    And I pick "LEGO"
    Then 2 of 3 wishes are placed
    And I see a comparison between "Bike" and "Headphones"
    When I pick "Headphones"
    Then I see a comparison between "LEGO" and "Headphones"
    And the wishlist order is "LEGO, Bike, Headphones"
    When I pick "LEGO"
    Then ranking is complete
    And the wishlist order is "LEGO, Headphones, Bike"

  Scenario: Continue the same ranking after viewing the list
    Given the wishlist order is "Bike, LEGO, Headphones"
    When I open the wishlist
    And I pick "LEGO"
    And I select "Visa önskelistan"
    Then the comparison is hidden
    When I continue ranking
    Then 2 of 3 wishes are placed
    And I see a comparison between "Bike" and "Headphones"

  Scenario: Open the ranked view from a comparison
    Given I am comparing wishes in a wishlist with twelve wishes
    When I select "Visa önskelistan"
    Then I see ranks 1 through 12 in order
    And the wishes appear in a single column
    And all wishes use the same card style
    And the comparison is hidden
