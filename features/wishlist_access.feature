Feature: Open a child's wishlist

  Scenario: Choose a child on the home page
    Given the app has a child named "Alex"
    When I open the app home page
    Then I can select Alex's child card immediately

  Scenario: Make wishlists for different children and occasions
    When I create child cards for "Alex" and "Sam"
    And I create the wishlists "Jul" and "Födelsedag" for "Alex"
    And I create the wishlist "Jul" for "Sam"
    Then Alex's card opens a screen with "Jul" and "Födelsedag"
    And Sam's card opens a screen with only "Jul"

  Scenario: Move from the child cards to the comparison screen
    Given Alex has a wishlist with at least two wishes
    When I select Alex's child card
    Then I see large cards for Alex's wishlists
    When I select one of those cards
    Then I see the heading "Välj din favorit" and two wishes from that wishlist
    And the ranked list is hidden
    And each of the two screen transitions uses a 300 ms fade

  Scenario: Open a wishlist with too few wishes to compare
    Given the wishlist contains fewer than two wishes
    When I open the wishlist
    Then I can add a wish
    And no comparison is shown
