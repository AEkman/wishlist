@wip
Feature: Unlock the app with a family PIN

  Scenario: Ask for the PIN before opening the app
    Given I have not unlocked the app in this browser
    When I open the home page or a direct link to a screen in the app
    Then the app asks for the family PIN before showing that screen

  Scenario: Use one PIN entry to access all wishlists in the browser
    Given the app has asked for the family PIN
    When I enter the configured family PIN
    Then I can open the app
    When I reopen the app in the same browser and select another child's wishlist
    Then the wishlist opens without another PIN prompt

  Scenario: Retry after an incorrect PIN
    Given the app has asked for the family PIN
    When I enter an incorrect PIN
    Then the app keeps the requested screen locked
    And I can try the PIN again

  Scenario: Open a shared wishlist without a PIN
    Given I have not unlocked the app in this browser
    And I have a link to a shared wishlist
    When I open the share link
    Then I see the shared wishlist and can mark wishes as bought
    When I open the app home page
    Then the app asks for the family PIN
