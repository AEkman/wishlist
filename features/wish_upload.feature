Feature: Add and edit wishes

  Scenario: Add a wish without a description
    Given Alex's wishlist is empty
    When I upload a picture with the title "LEGO" and leave the description blank
    Then "LEGO" is the first wish on Alex's wishlist
    And no comparison is shown

  Scenario: Add a wish with a description
    Given I have opened Alex's wishlist
    When I upload a picture with the title "LEGO" and the description "The space set"
    Then Alex's wish includes the picture, title, and description

  Scenario: Add a wish using the phone camera
    Given I have opened Alex's wishlist
    When I open the add-wish form
    Then I can take a picture or choose an existing picture
    When I submit a camera picture with the title "LEGO"
    Then Alex's wish includes the picture, title, and description

  Scenario Outline: Compare a new wish with a limited number of different opponents
    Given Alex's wishlist has <Existing wishes> wishes
    When I add a wish with a picture and title
    Then the upload requires <Picks> picks
    And the new wish stays on the left in each upload comparison
    And each opponent appears at most once in the upload comparisons

    Examples:
      | Existing wishes | Picks |
      | 0               | 0     |
      | 1               | 1     |
      | 2               | 2     |
      | 3               | 2     |
      | 4               | 3     |
      | 9               | 3     |

  Scenario: Try a higher-ranked opponent after a win
    Given the wishlist order is "Bike, LEGO, Skates, Ball, New wish"
    And the first upload comparison randomly selects "Ball" as the opponent
    When I pick "New wish"
    Then the wishlist order is "Bike, LEGO, Skates, New wish, Ball"
    And the next opponent is one of "Bike, LEGO, Skates"

  Scenario: Try a lower-ranked opponent after a loss
    Given the wishlist order is "Bike, LEGO, Skates, Ball, New wish"
    And the first upload comparison randomly selects "LEGO" as the opponent
    When I pick "LEGO"
    Then the wishlist order is "Bike, LEGO, Skates, Ball, New wish"
    And the next opponent is one of "Skates, Ball"

  Scenario: Use an untried opponent when no higher-ranked opponent remains
    Given the wishlist order is "Bike, LEGO, Skates, Ball, New wish"
    And the first upload comparison randomly selects "Bike" as the opponent
    When I pick "New wish"
    Then "New wish" is first
    And the next opponent is one of "LEGO, Skates, Ball"

  Scenario: Use an untried opponent when no lower-ranked opponent remains
    Given the wishlist order is "Bike, LEGO, Skates, Ball, New wish"
    And the first upload comparison randomly selects "Ball" as the opponent
    When I pick "Ball"
    Then the next opponent is one of "Bike, LEGO, Skates"

  Scenario: Edit a wish and keep its rank and purchase mark
    Given "LEGO" is ranked third and marked as bought by "Anna" on the shared list
    When I change the wish's picture, title, and description
    Then the updated wish is still ranked third
    And the shared list still shows it as bought by "Anna"
    And the edit starts no upload comparisons
