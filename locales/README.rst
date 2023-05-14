=====================================================
Here are the basic steps for setting up translations:
=====================================================


Create a translations template file:
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Run the following command to extract the strings from your source code and generate the corresponding ``.pot`` (Portable Object Template) file:

- You will need to do this every time you make a change in your source code.

.. code-block:: bash

    poetry run pybabel extract --input-dirs=./smudge -o locales/bot.pot --no-wrap --omit-header

This will create a ``bot.pot`` file that contains all the translatable strings in your application.

Create translation files:
~~~~~~~~~~~~~~~~~~~~~~~~~
| Once you have the translation template file, you can create a ``.po`` (Portable Object) translation file for each language supported by your bot.
| Run the following command to create a .po file for the language ``pt_BR`` *(Brazilian Portuguese)*, for example:

- **⚠️ WARNING:** Only do this when there is no ``.po`` file for the chosen language, if you do this with an existing ``.po`` file, you will delete all translations in that file.

.. code-block:: bash

    poetry run pybabel init -i locales/bot.pot -d locales -D bot -l pt_BR --no-wrap

This will create a ``.po`` file inside the folder for the chosen language. You can translate this file using a text editor.

Update translated translation files:
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
| When you make changes to the .pot file (whenever you generate the message file), you need to update the corresponding ``.po`` files.
| You can do this by running the following command:

.. code-block:: bash

    poetry run pybabel update -i locales/bot.pot -d locales -D bot --omit-header --no-wrap

This command will update the ``.po`` files in the translations directory with any new or changed strings from the bot.pot file.

Compile the translation files:
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
| After you have translated the strings, you need to compile the translation files into binary ``.mo`` (Machine Object) format.
| To do this, running the following command:

.. code-block:: bash

    poetry run pybabel compile -d locales -D bot --statistics

This will create a ``.mo`` file inside the folder for the language you have chosen. With this file, your bot can use it to display the translated strings.
