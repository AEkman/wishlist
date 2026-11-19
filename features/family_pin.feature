@wip
Feature: Unlock the app with a family PIN

  Only shared wishlist links are public.

  Scenario Outline: Ask for the PIN before showing an app screen
    Given I have not unlocked the app in this browser
    When I open <Screen> directly
    Then the app asks for the family PIN before showing that screen

    Examples:
      | Screen                  |
      | the home page           |
      | a child's wishlist menu |
      | a comparison screen     |
      | a ranked wishlist view  |
      | an add-wish form        |
      | an edit-wish form       |

  Scenario: Use one PIN entry for the unlocked browser session
    Given the app has asked for the family PIN
    When I enter the configured family PIN
    Then I can open the requested screen
    When I select another child's wishlist in the same unlocked browser session
    Then I can open that wishlist without another PIN prompt

  Scenario: Retry after an incorrect PIN
    Given the app has asked for the family PIN
    When I enter an incorrect PIN
    Then the app keeps the requested screen locked
    And I can try the PIN again

  Scenario: Open a shared wishlist without a PIN
    Given I have not unlocked the app in this browser
    And I have a link to a shared wishlist
    When I open the share link
    Then I see the full shared wishlist in rank order
    And I have no controls to add, edit, or rank wishes
    When I open the app home page
    Then the app asks for the family PIN
