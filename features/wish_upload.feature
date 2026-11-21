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

  Scenario Outline: Place a new wish by binary insertion
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
      | 9               | 4     |

  Scenario: Place a new favorite with midpoint comparisons
    Given the wishlist order is "Bike, LEGO, Skates, Ball, New wish"
    And the first upload comparison selects "Skates" as the opponent
    When I pick "New wish"
    Then the wishlist order is "Bike, LEGO, Skates, Ball, New wish"
    And the next opponent is "LEGO"
    When I pick "New wish"
    Then the next opponent is "Bike"
    When I pick "New wish"
    Then "New wish" is first
    And no comparison is shown

  Scenario: Place a new wish between two existing wishes
    Given the wishlist order is "Bike, LEGO, Skates, Ball, New wish"
    And the first upload comparison selects "Skates" as the opponent
    When I pick "Skates"
    Then the next opponent is "Ball"
    When I pick "New wish"
    Then the wishlist order is "Bike, LEGO, Skates, New wish, Ball"
    And no comparison is shown

  Scenario: Edit a wish and keep its rank and purchase mark
    Given "LEGO" is ranked third and marked as bought by "Anna" on the shared list
    When I change the wish's picture, title, and description
    Then the updated wish is still ranked third
    And the shared list still shows it as bought by "Anna"
    And the edit starts no upload comparisons
