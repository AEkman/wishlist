Feature: Rank wishes by comparing two at a time

  Scenario: Draw a random pair from the entire wishlist
    Given the wishlist has more than ten wishes
    When I open the wishlist
    Then I see two different wishes drawn at random from the entire wishlist
    And the ranked list is hidden

  Scenario: Compare a wishlist that has exactly two wishes
    Given the wishlist contains only "Bike" and "LEGO"
    When I open the wishlist
    Then I see both wishes in random left-to-right order
    And I can pick either wish

  Scenario: Move a lower-ranked winner immediately above the loser
    Given the wishlist order is "Bike, LEGO, Headphones"
    And I am comparing "Bike" and "Headphones"
    When I pick "Headphones"
    Then the wishlist order is immediately "Headphones, Bike, LEGO"

  Scenario: Keep the order when the higher-ranked wish wins
    Given the wishlist order is "Bike, LEGO, Headphones"
    And I am comparing "Bike" and "Headphones"
    When I pick "Bike"
    Then the wishlist order is "Bike, LEGO, Headphones"

  Scenario: Open the ranked view from a comparison
    Given I am comparing wishes in a wishlist with twelve wishes
    When I select "Visa önskelistan"
    Then I see ranks 1 through 10 in order
    And the first five wishes have larger cards than the next five
    And the comparison is hidden
