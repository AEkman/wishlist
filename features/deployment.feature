Feature: Start and stop the app with Docker Compose

  Local Godog tests use a fresh in-memory database for each scenario.
  The Docker Compose deployment stores wishlist data and uploads on persistent storage.

  Scenario: Start the app from the Makefile
    Given Docker Compose is available on the server
    When I run "make run" from the project directory
    Then the app starts in the background
    And I can open its home page

  Scenario: Stop and restart the app with the Makefile
    Given the app is running
    And a wishlist has a ranked wish with an uploaded picture and a purchase mark
    When I run "make stop" from the project directory
    Then the app stops serving requests
    When I run "make start" from the project directory
    Then the wish is still in the same rank with its uploaded picture
    And the shared list still shows its purchase mark
