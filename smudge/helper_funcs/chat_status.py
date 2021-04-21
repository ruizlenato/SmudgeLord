#    SmudgeLord (A telegram bot project)
#    Copyright (C) 2017-2019 Paul Larsen
#    Copyright (C) 2019-2021 A Haruka Aita and Intellivoid Technologies project
#    Copyright (C) 2021 Renatoh

#    This program is free software: you can redistribute it and/or modify
#    it under the terms of the GNU Affero General Public License as published by
#    the Free Software Foundation, either version 3 of the License, or
#    (at your option) any later version.

#    You should have received a copy of the GNU Affero General Public License
#    along with this program.  If not, see <https://www.gnu.org/licenses/>.

from functools import wraps
from typing import Optional

from telegram import User, Chat, ChatMember, Update, Bot

from smudge import CallbackContext, DEL_CMDS, SUDO_USERS, WHITELIST_USERS
from smudge.modules.translations.strings import tld


def can_delete(chat: Chat, bot_id: int) -> bool:
    return chat.get_member(bot_id).can_delete_messages


def is_user_ban_protected(chat: Chat,
                          user_id: int,
                          member: ChatMember = None) -> bool:
    if chat.type == 'private' \
            or chat.all_members_are_administrators:
        return True

    if not member:
        member = chat.get_member(user_id)
    return member.status in ('administrator', 'creator')


def is_user_admin(chat: Chat, user_id: int, member: ChatMember = None) -> bool:
    if chat.type == 'private' \
            or chat.all_members_are_administrators:
        return True

    if not member:
        member = chat.get_member(user_id)
    return member.status in ('administrator', 'creator')


def is_bot_admin(chat: Chat,
                 bot_id: int,
                 bot_member: ChatMember = None) -> bool:
    if chat.type == 'private' \
            or chat.all_members_are_administrators:
        return True

    if not bot_member:
        bot_member = chat.get_member(bot_id)
    return bot_member.status in ('administrator', 'creator')


def is_user_in_chat(chat: Chat, user_id: int) -> bool:
    member = chat.get_member(user_id)
    return member.status not in ('left', 'kicked')


def bot_can_delete(func):
    @wraps(func)
    def delete_rights(update: Update, context: CallbackContext, *args,
                      **kwargs):
        bot = context.bot
        if can_delete(update.effective_chat, bot.id):
            return func(update, context, *args, **kwargs)
        else:
            update.effective_message.reply_text(
                "I can't delete messages here! "
                "Make sure I'm admin and can delete other user's messages.")

    return delete_rights


def can_pin(func):
    @wraps(func)
    def pin_rights(update: Update, context: CallbackContext, *args, **kwargs):
        bot = context.bot
        if update.effective_chat.get_member(bot.id).can_pin_messages:
            return func(update, context, *args, **kwargs)
        else:
            update.effective_message.reply_text(
                "I can't pin messages here! "
                "Make sure I'm admin and can pin messages.")

    return pin_rights


def can_promote(func):
    @wraps(func)
    def promote_rights(update: Update, context: CallbackContext, *args,
                       **kwargs):
        bot = context.bot
        if update.effective_chat.get_member(bot.id).can_promote_members:
            return func(update, context, *args, **kwargs)
        else:
            update.effective_message.reply_text(
                "I can't promote/demote people here! "
                "Make sure I'm admin and can appoint new admins.")

    return promote_rights


def can_restrict(func):
    @wraps(func)
    def promote_rights(update: Update, context: CallbackContext, *args,
                       **kwargs):
        bot = context.bot
        if update.effective_chat.get_member(bot.id).can_restrict_members:
            return func(update, context, *args, **kwargs)
        else:
            update.effective_message.reply_text(
                "I can't restrict people here! "
                "Make sure I'm admin and can appoint new admins.")

    return promote_rights


def bot_admin(func):
    @wraps(func)
    def is_admin(update: Update, context: CallbackContext, *args, **kwargs):
        chat = update.effective_chat

        if is_bot_admin(update.effective_chat, context.bot.id):
            return func(update, context, *args, **kwargs)
        else:
            update.effective_message.reply_text(
                tld(chat.id, 'helpers_bot_not_admin'))

    return is_admin


def user_admin(func):
    @wraps(func)
    def is_admin(update: Update, context: CallbackContext, *args, **kwargs):
        bot = context.bot
        user = update.effective_user  # type: Optional[User]
        if user and is_user_admin(update.effective_chat, user.id):
            return func(update, context, *args, **kwargs)

        elif not user:
            pass

        else:
            update.effective_message.delete()

    return is_admin


def user_admin_no_reply(func):
    @wraps(func)
    def is_admin(update: Update, context: CallbackContext, *args, **kwargs):
        bot = context.bot
        user = update.effective_user  # type: Optional[User]
        if user and is_user_admin(update.effective_chat, user.id):
            return func(update, context, *args, **kwargs)

        elif not user:
            pass

        elif DEL_CMDS and " " not in update.effective_message.text:
            update.effective_message.delete()

    return is_admin


def user_not_admin(func):
    @wraps(func)
    def is_not_admin(update: Update, context: CallbackContext, *args,
                     **kwargs):
        bot = context.bot
        user = update.effective_user  # type: Optional[User]
        if user and not is_user_admin(update.effective_chat, user.id):
            return func(update, context, *args, **kwargs)

    return is_not_admin


def user_can_ban(func):
    @wraps(func)
    def user_perm_ban(update: Update, context: CallbackContext, *args, **kwargs):
        user = update.effective_user.id
        chat = update.effective_chat
        member = update.effective_chat.get_member(user)

        if not (member.can_restrict_members or
                member.status == "creator"):
            update.effective_message.reply_text(
                tld(chat.id, "admin_ban_perm_false"))
            return ""
        return func(update, context, *args, **kwargs)

    return user_perm_ban


def user_can_kick(func):
    @wraps(func)
    def user_perm_kick(update: Update, context: CallbackContext, *args, **kwargs):
        user = update.effective_user.id
        chat = update.effective_chat
        member = update.effective_chat.get_member(user)

        if not (member.can_restrict_members or
                member.status == "creator"):
            update.effective_message.reply_text(
                tld(chat.id, "admin_kick_perm_false"))
            return ""
        return func(update, context, *args, **kwargs)

    return user_perm_kick


def user_can_warn(func):
    @wraps(func)
    def user_perm_warn(update: Update, context: CallbackContext, *args, **kwargs):
        user = update.effective_user.id
        chat = update.effective_chat
        member = update.effective_chat.get_member(user)

        if not (member.can_restrict_members or
                member.status == "creator"):
            update.effective_message.reply_text(
                tld(chat.id, "admin_warn_perm_false"))
            return ""
        return func(update, context, *args, **kwargs)

    return user_perm_warn


def user_can_promote(func):
    @wraps(func)
    def user_perm_promote(update: Update, context: CallbackContext, *args, **kwargs):
        user = update.effective_user.id
        chat = update.effective_chat
        member = update.effective_chat.get_member(user)

        if not (member.can_promote_members or
                member.status == "creator"):
            update.effective_message.reply_text(
                tld(chat.id, "admin_promote_perm_false"))
            return ""
        return func(update, context, *args, **kwargs)

    return user_perm_promote


def user_can_pin(func):
    @wraps(func)
    def user_perm_pin(update: Update, context: CallbackContext, *args, **kwargs):
        user = update.effective_user.id
        chat = update.effective_chat
        member = update.effective_chat.get_member(user)

        if not (member.can_pin_messages or
                member.status == "creator"):
            update.effective_message.reply_text(
                tld(chat.id, "admin_pin_perm_false"))
            return ""
        return func(update, context, *args, **kwargs)

    return user_perm_pin


def user_can_changeinfo(func):
    @wraps(func)
    def user_perm_changeinfo_group(update: Update, context: CallbackContext, *args, **kwargs):
        user = update.effective_user.id
        chat = update.effective_chat
        member = update.effective_chat.get_member(user)

        if not (member.can_change_info or member.status == "creator"):
            update.effective_message.reply_text(
                tld(chat.id, "admin_changeinfo_perm_false"))
            return ""
        return func(update, context, *args, **kwargs)

    return user_perm_changeinfo_group
